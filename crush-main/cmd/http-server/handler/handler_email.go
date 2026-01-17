package handler

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/rolling1314/rolling-crush/auth"
	"github.com/rolling1314/rolling-crush/infra/email"
)

// handleSendVerificationCode sends a verification code to the user's email
func (s *Server) handleSendVerificationCode(c *gin.Context) {
	var req SendVerificationCodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}

	// Validate code type
	var codeType email.CodeType
	switch req.Type {
	case "register":
		codeType = email.CodeTypeRegister
		// Check if email already registered
		_, err := s.userService.GetByEmail(c.Request.Context(), req.Email)
		if err == nil {
			c.JSON(http.StatusBadRequest, ErrorResponse{Error: "该邮箱已注册"})
			return
		}
	case "reset_password":
		codeType = email.CodeTypeResetPassword
		// Check if email exists
		_, err := s.userService.GetByEmail(c.Request.Context(), req.Email)
		if err != nil {
			c.JSON(http.StatusBadRequest, ErrorResponse{Error: "该邮箱未注册"})
			return
		}
	default:
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "无效的验证码类型"})
		return
	}

	// Generate verification code
	code, err := s.emailService.GenerateCode()
	if err != nil {
		slog.Error("Failed to generate verification code", "error", err)
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "生成验证码失败"})
		return
	}

	// Store the code
	s.emailService.StoreCode(req.Email, code, codeType)

	// Send the email
	if err := s.emailService.SendVerificationCode(req.Email, code, codeType); err != nil {
		slog.Error("Failed to send verification email", "error", err, "email", req.Email)
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "发送验证码失败，请稍后重试"})
		return
	}

	slog.Info("Verification code sent", "email", req.Email, "type", req.Type)
	c.JSON(http.StatusOK, SendVerificationCodeResponse{
		Success: true,
		Message: "验证码已发送到您的邮箱",
	})
}

// handleVerifyEmailCode verifies the email verification code
func (s *Server) handleVerifyEmailCode(c *gin.Context) {
	var req VerifyEmailCodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}

	var codeType email.CodeType
	switch req.Type {
	case "register":
		codeType = email.CodeTypeRegister
	case "reset_password":
		codeType = email.CodeTypeResetPassword
	default:
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "无效的验证码类型"})
		return
	}

	if !s.emailService.VerifyCode(req.Email, req.Code, codeType) {
		c.JSON(http.StatusBadRequest, VerifyEmailCodeResponse{
			Success: false,
			Message: "验证码无效或已过期",
		})
		return
	}

	c.JSON(http.StatusOK, VerifyEmailCodeResponse{
		Success: true,
		Message: "验证码验证成功",
	})
}

// handleRegisterWithCode handles user registration with email verification code
func (s *Server) handleRegisterWithCode(c *gin.Context) {
	var req RegisterWithCodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}

	// Verify the code first
	if !s.emailService.VerifyCode(req.Email, req.Code, email.CodeTypeRegister) {
		c.JSON(http.StatusBadRequest, LoginResponse{
			Success: false,
			Message: "验证码无效或已过期",
		})
		return
	}

	// Create user
	user, err := s.userService.Create(c.Request.Context(), req.Username, req.Email, req.Password)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}

	// Delete the used code
	s.emailService.DeleteCode(req.Email)

	// Generate token
	token, err := auth.GenerateToken(user.ID, user.Username)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "生成令牌失败"})
		return
	}

	slog.Info("User registered with email verification", "email", req.Email, "username", req.Username)
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

// handleForgotPassword initiates the password reset process
func (s *Server) handleForgotPassword(c *gin.Context) {
	var req ForgotPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}

	// Check if user exists
	_, err := s.userService.GetByEmail(c.Request.Context(), req.Email)
	if err != nil {
		// Don't reveal if email exists or not for security
		c.JSON(http.StatusOK, SendVerificationCodeResponse{
			Success: true,
			Message: "如果该邮箱已注册，验证码将发送到您的邮箱",
		})
		return
	}

	// Generate verification code
	code, err := s.emailService.GenerateCode()
	if err != nil {
		slog.Error("Failed to generate verification code", "error", err)
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "生成验证码失败"})
		return
	}

	// Store the code
	s.emailService.StoreCode(req.Email, code, email.CodeTypeResetPassword)

	// Send the email
	if err := s.emailService.SendVerificationCode(req.Email, code, email.CodeTypeResetPassword); err != nil {
		slog.Error("Failed to send password reset email", "error", err, "email", req.Email)
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "发送验证码失败，请稍后重试"})
		return
	}

	slog.Info("Password reset code sent", "email", req.Email)
	c.JSON(http.StatusOK, SendVerificationCodeResponse{
		Success: true,
		Message: "验证码已发送到您的邮箱",
	})
}

// handleResetPassword resets the user's password
func (s *Server) handleResetPassword(c *gin.Context) {
	var req ResetPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}

	// Verify the code first
	if !s.emailService.VerifyCode(req.Email, req.Code, email.CodeTypeResetPassword) {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "验证码无效或已过期"})
		return
	}

	// Get user
	user, err := s.userService.GetByEmail(c.Request.Context(), req.Email)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "用户不存在"})
		return
	}

	// Update password
	if err := s.userService.UpdatePassword(c.Request.Context(), user.ID, req.NewPassword); err != nil {
		slog.Error("Failed to update password", "error", err, "email", req.Email)
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "重置密码失败"})
		return
	}

	// Delete the used code
	s.emailService.DeleteCode(req.Email)

	slog.Info("Password reset successful", "email", req.Email)
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "密码重置成功",
	})
}
