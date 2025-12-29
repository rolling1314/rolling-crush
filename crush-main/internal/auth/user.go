package auth

import (
	"crypto/sha256"
	"encoding/hex"
	"sync"
)

// User represents a user in the system
type User struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	Password string `json:"-"` // Password hash, never expose in JSON
}

// UserStore is a simple in-memory user store
// In production, this should be replaced with a database
type UserStore struct {
	users map[string]*User
	mu    sync.RWMutex
}

var (
	// Global user store instance
	store = &UserStore{
		users: make(map[string]*User),
	}
)

func init() {
	// Create a default admin user for testing
	// In production, users should be created through a proper registration process
	store.CreateUser("admin", "admin123")
	store.CreateUser("user", "password123")
}

// GetUserStore returns the global user store instance
func GetUserStore() *UserStore {
	return store
}

// CreateUser creates a new user with hashed password
func (s *UserStore) CreateUser(username, password string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	// Check if user already exists
	if _, exists := s.users[username]; exists {
		return ErrUserAlreadyExists
	}
	
	// Hash the password
	hashedPassword := hashPassword(password)
	
	user := &User{
		ID:       generateUserID(username),
		Username: username,
		Password: hashedPassword,
	}
	
	s.users[username] = user
	return nil
}

// Authenticate validates username and password
func (s *UserStore) Authenticate(username, password string) (*User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	user, exists := s.users[username]
	if !exists {
		return nil, ErrInvalidCredentials
	}
	
	hashedPassword := hashPassword(password)
	if user.Password != hashedPassword {
		return nil, ErrInvalidCredentials
	}
	
	return user, nil
}

// GetUser retrieves a user by username
func (s *UserStore) GetUser(username string) (*User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	user, exists := s.users[username]
	if !exists {
		return nil, ErrUserNotFound
	}
	
	return user, nil
}

// hashPassword creates a SHA-256 hash of the password
// In production, use bcrypt or argon2 instead
func hashPassword(password string) string {
	hash := sha256.Sum256([]byte(password))
	return hex.EncodeToString(hash[:])
}

// generateUserID generates a unique user ID
func generateUserID(username string) string {
	hash := sha256.Sum256([]byte(username))
	return hex.EncodeToString(hash[:16])
}

var (
	ErrUserAlreadyExists  = &AuthError{Message: "user already exists"}
	ErrInvalidCredentials = &AuthError{Message: "invalid credentials"}
	ErrUserNotFound       = &AuthError{Message: "user not found"}
)

// AuthError represents an authentication error
type AuthError struct {
	Message string
}

func (e *AuthError) Error() string {
	return e.Message
}

