package http

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"

	"github.com/charmbracelet/crush/auth"
	"github.com/gin-gonic/gin"
)

// GitHub OAuth configuration
var (
	githubClientID     = getEnvOrDefault("GITHUB_CLIENT_ID", "Ov23liHJsgAHhcbppKO3")
	githubClientSecret = getEnvOrDefault("GITHUB_CLIENT_SECRET", "35e742c45cae57f001c5a3a6f6cf058a4338d1b4")
	githubRedirectURI  = getEnvOrDefault("GITHUB_REDIRECT_URI", "http://localhost:8081/api/auth/github/callback")
	frontendURL        = getEnvOrDefault("FRONTEND_URL", "http://localhost:8080")
)

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// GitHubUser represents GitHub user info from API
type GitHubUser struct {
	ID        int64  `json:"id"`
	Login     string `json:"login"`
	Email     string `json:"email"`
	AvatarURL string `json:"avatar_url"`
	Name      string `json:"name"`
}

// GitHubTokenResponse represents GitHub OAuth token response
type GitHubTokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	Scope       string `json:"scope"`
}

// handleRegister handles user registration
func (s *Server) handleRegister(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}

	user, err := s.userService.Create(c.Request.Context(), req.Username, req.Email, req.Password)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}

	token, err := auth.GenerateToken(user.ID, user.Username)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "Failed to generate token"})
		return
	}

	c.JSON(http.StatusOK, LoginResponse{
		Success: true,
		Token:   token,
		User: &UserInfo{
			ID:       user.ID,
			Username: user.Username,
			Email:    user.Email,
		},
	})
}

// handleLogin handles user login
func (s *Server) handleLogin(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}

	user, err := s.userService.VerifyPassword(c.Request.Context(), req.Email, req.Password)
	if err != nil {
		c.JSON(http.StatusUnauthorized, LoginResponse{
			Success: false,
			Message: "Invalid email or password",
		})
		return
	}

	token, err := auth.GenerateToken(user.ID, user.Username)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "Failed to generate token"})
		return
	}

	c.JSON(http.StatusOK, LoginResponse{
		Success: true,
		Token:   token,
		User: &UserInfo{
			ID:       user.ID,
			Username: user.Username,
			Email:    user.Email,
		},
	})
}

// handleVerify handles token verification
func (s *Server) handleVerify(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"valid": true})
}

// handleGitHubLogin redirects to GitHub OAuth authorization page
func (s *Server) handleGitHubLogin(c *gin.Context) {
	state := generateRandomState()
	// Store state in a cookie for CSRF protection
	c.SetCookie("oauth_state", state, 600, "/", "", false, true)

	authURL := fmt.Sprintf(
		"https://github.com/login/oauth/authorize?client_id=%s&redirect_uri=%s&scope=user:email&state=%s",
		githubClientID,
		githubRedirectURI,
		state,
	)

	c.JSON(http.StatusOK, gin.H{
		"auth_url": authURL,
	})
}

// handleGitHubCallback handles the GitHub OAuth callback
func (s *Server) handleGitHubCallback(c *gin.Context) {
	code := c.Query("code")
	state := c.Query("state")

	if code == "" {
		c.Redirect(http.StatusTemporaryRedirect, frontendURL+"?error=missing_code")
		return
	}

	// Verify state for CSRF protection (optional check)
	storedState, _ := c.Cookie("oauth_state")
	if state != "" && storedState != "" && state != storedState {
		c.Redirect(http.StatusTemporaryRedirect, frontendURL+"?error=invalid_state")
		return
	}

	// Exchange code for access token
	accessToken, err := exchangeGitHubCode(code)
	if err != nil {
		c.Redirect(http.StatusTemporaryRedirect, frontendURL+"?error=exchange_failed")
		return
	}

	// Get GitHub user info
	githubUser, err := getGitHubUser(accessToken)
	if err != nil {
		c.Redirect(http.StatusTemporaryRedirect, frontendURL+"?error=user_info_failed")
		return
	}

	// If GitHub doesn't provide email, use login@github.com
	email := githubUser.Email
	if email == "" {
		email = fmt.Sprintf("%s@github.local", githubUser.Login)
	}

	// Try to find existing user by email
	existingUser, err := s.userService.GetByEmail(c.Request.Context(), email)
	if err != nil {
		// User doesn't exist, create new user
		// Generate random password since they'll use OAuth
		randomPassword := generateRandomPassword()
		username := githubUser.Login
		if githubUser.Name != "" {
			username = githubUser.Name
		}

		newUser, err := s.userService.Create(c.Request.Context(), username, email, randomPassword)
		if err != nil {
			// If username conflict, try with GitHub ID suffix
			username = fmt.Sprintf("%s_%d", githubUser.Login, githubUser.ID)
			newUser, err = s.userService.Create(c.Request.Context(), username, email, randomPassword)
			if err != nil {
				c.Redirect(http.StatusTemporaryRedirect, frontendURL+"?error=create_user_failed")
				return
			}
		}

		// Update avatar URL if provided
		if githubUser.AvatarURL != "" {
			newUser.AvatarURL = sql.NullString{String: githubUser.AvatarURL, Valid: true}
			s.userService.Update(c.Request.Context(), newUser)
		}

		existingUser = newUser
	}

	// Generate JWT token
	token, err := auth.GenerateToken(existingUser.ID, existingUser.Username)
	if err != nil {
		c.Redirect(http.StatusTemporaryRedirect, frontendURL+"?error=token_failed")
		return
	}

	// Redirect to frontend with token and user info (URL encoded)
	redirectURL := fmt.Sprintf("%s/auth/github/callback?token=%s&user_id=%s&username=%s&email=%s&avatar_url=%s",
		frontendURL,
		url.QueryEscape(token),
		url.QueryEscape(existingUser.ID),
		url.QueryEscape(existingUser.Username),
		url.QueryEscape(existingUser.Email),
		url.QueryEscape(existingUser.AvatarURL.String),
	)

	c.Redirect(http.StatusTemporaryRedirect, redirectURL)
}

func exchangeGitHubCode(code string) (string, error) {
	client := &http.Client{}

	reqURL := fmt.Sprintf(
		"https://github.com/login/oauth/access_token?client_id=%s&client_secret=%s&code=%s",
		githubClientID,
		githubClientSecret,
		code,
	)

	req, err := http.NewRequest("POST", reqURL, nil)
	if err != nil {
		return "", err
	}

	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var tokenResp GitHubTokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return "", err
	}

	if tokenResp.AccessToken == "" {
		return "", fmt.Errorf("no access token in response: %s", string(body))
	}

	return tokenResp.AccessToken, nil
}

func getGitHubUser(accessToken string) (*GitHubUser, error) {
	client := &http.Client{}

	req, err := http.NewRequest("GET", "https://api.github.com/user", nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var user GitHubUser
	if err := json.Unmarshal(body, &user); err != nil {
		return nil, err
	}

	// If email is empty, try to get from emails endpoint
	if user.Email == "" {
		user.Email, _ = getGitHubPrimaryEmail(accessToken)
	}

	return &user, nil
}

func getGitHubPrimaryEmail(accessToken string) (string, error) {
	client := &http.Client{}

	req, err := http.NewRequest("GET", "https://api.github.com/user/emails", nil)
	if err != nil {
		return "", err
	}

	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var emails []struct {
		Email    string `json:"email"`
		Primary  bool   `json:"primary"`
		Verified bool   `json:"verified"`
	}

	if err := json.Unmarshal(body, &emails); err != nil {
		return "", err
	}

	for _, e := range emails {
		if e.Primary && e.Verified {
			return e.Email, nil
		}
	}

	for _, e := range emails {
		if e.Verified {
			return e.Email, nil
		}
	}

	return "", fmt.Errorf("no verified email found")
}

func generateRandomState() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func generateRandomPassword() string {
	b := make([]byte, 32)
	rand.Read(b)
	return hex.EncodeToString(b)
}
