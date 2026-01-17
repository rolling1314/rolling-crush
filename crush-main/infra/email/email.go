package email

import (
	"crypto/rand"
	"crypto/tls"
	"fmt"
	"math/big"
	"net/smtp"
	"sync"
	"time"

	"github.com/rolling1314/rolling-crush/pkg/config"
)

// VerificationCode stores a verification code with its expiration time
type VerificationCode struct {
	Code      string
	ExpiresAt time.Time
	Type      CodeType // "register" or "reset_password"
}

// CodeType represents the type of verification code
type CodeType string

const (
	CodeTypeRegister      CodeType = "register"
	CodeTypeResetPassword CodeType = "reset_password"
)

// Service provides email functionality
type Service struct {
	config     *config.EmailConfig
	codes      map[string]*VerificationCode // email -> code
	codesMutex sync.RWMutex
}

// NewService creates a new email service
func NewService(cfg *config.EmailConfig) *Service {
	return &Service{
		config: cfg,
		codes:  make(map[string]*VerificationCode),
	}
}

// GenerateCode generates a 6-digit verification code
func (s *Service) GenerateCode() (string, error) {
	const digits = "0123456789"
	code := make([]byte, 6)
	for i := range code {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(digits))))
		if err != nil {
			return "", err
		}
		code[i] = digits[n.Int64()]
	}
	return string(code), nil
}

// StoreCode stores a verification code for an email
func (s *Service) StoreCode(email string, code string, codeType CodeType) {
	s.codesMutex.Lock()
	defer s.codesMutex.Unlock()

	expireMinutes := s.config.CodeExpire
	if expireMinutes == 0 {
		expireMinutes = 5 // default 5 minutes
	}

	s.codes[email] = &VerificationCode{
		Code:      code,
		ExpiresAt: time.Now().Add(time.Duration(expireMinutes) * time.Minute),
		Type:      codeType,
	}
}

// VerifyCode verifies a code for an email
func (s *Service) VerifyCode(email string, code string, codeType CodeType) bool {
	s.codesMutex.RLock()
	defer s.codesMutex.RUnlock()

	stored, exists := s.codes[email]
	if !exists {
		return false
	}

	if time.Now().After(stored.ExpiresAt) {
		return false
	}

	if stored.Type != codeType {
		return false
	}

	return stored.Code == code
}

// DeleteCode removes a verification code
func (s *Service) DeleteCode(email string) {
	s.codesMutex.Lock()
	defer s.codesMutex.Unlock()
	delete(s.codes, email)
}

// SendVerificationCode sends a verification code to the email
func (s *Service) SendVerificationCode(toEmail string, code string, codeType CodeType) error {
	var subject, bodyText string

	switch codeType {
	case CodeTypeRegister:
		subject = "欢迎注册 - 验证码"
		bodyText = fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head>
  <meta charset="UTF-8">
  <style>
    body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; margin: 0; padding: 0; background: #0a0a0a; }
    .container { max-width: 600px; margin: 0 auto; background: linear-gradient(180deg, #0f1419 0%%, #0a0a0a 100%%); border-radius: 16px; overflow: hidden; border: 1px solid rgba(255,255,255,0.1); }
    .header { background: linear-gradient(135deg, #1a3a4a 0%%, #0f2833 100%%); padding: 32px; text-align: center; }
    .header h1 { color: #4fd1c5; margin: 0; font-size: 24px; font-weight: 600; }
    .content { padding: 40px 32px; color: #e2e8f0; }
    .greeting { font-size: 16px; margin-bottom: 24px; color: #a0aec0; }
    .message { font-size: 15px; line-height: 1.6; margin-bottom: 32px; color: #a0aec0; }
    .code-box { background: rgba(79, 209, 197, 0.1); border: 1px solid rgba(79, 209, 197, 0.3); border-radius: 12px; padding: 24px; text-align: center; margin: 24px 0; }
    .code { font-size: 36px; font-weight: 700; letter-spacing: 8px; color: #4fd1c5; font-family: 'SF Mono', Monaco, monospace; }
    .tips { background: rgba(255,255,255,0.05); border-radius: 8px; padding: 16px; margin-top: 24px; }
    .tips-title { color: #4fd1c5; font-weight: 600; margin-bottom: 12px; font-size: 14px; }
    .tips ul { margin: 0; padding-left: 20px; color: #718096; font-size: 13px; line-height: 1.8; }
    .footer { padding: 24px 32px; text-align: center; border-top: 1px solid rgba(255,255,255,0.05); }
    .footer p { color: #4a5568; font-size: 12px; margin: 0; }
  </style>
</head>
<body>
  <div class="container">
    <div class="header">
      <h1>欢迎注册</h1>
    </div>
    <div class="content">
      <p class="greeting">尊敬的用户：</p>
      <p class="message">您正在进行注册操作，请输入以下验证码完成验证：</p>
      <div class="code-box">
        <span class="code">%s</span>
      </div>
      <div class="tips">
        <p class="tips-title">安全提示：</p>
        <ul>
          <li>验证码有效期为5分钟</li>
          <li>请勿将验证码泄露给他人</li>
          <li>如非本人操作，请忽略此邮件</li>
        </ul>
      </div>
    </div>
    <div class="footer">
      <p>此邮件由系统自动发送，请勿回复</p>
    </div>
  </div>
</body>
</html>
`, code)
	case CodeTypeResetPassword:
		subject = "密码重置 - 验证码"
		bodyText = fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head>
  <meta charset="UTF-8">
  <style>
    body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; margin: 0; padding: 0; background: #0a0a0a; }
    .container { max-width: 600px; margin: 0 auto; background: linear-gradient(180deg, #0f1419 0%%, #0a0a0a 100%%); border-radius: 16px; overflow: hidden; border: 1px solid rgba(255,255,255,0.1); }
    .header { background: linear-gradient(135deg, #4a1a3a 0%%, #2d0f23 100%%); padding: 32px; text-align: center; }
    .header h1 { color: #f687b3; margin: 0; font-size: 24px; font-weight: 600; }
    .content { padding: 40px 32px; color: #e2e8f0; }
    .greeting { font-size: 16px; margin-bottom: 24px; color: #a0aec0; }
    .message { font-size: 15px; line-height: 1.6; margin-bottom: 32px; color: #a0aec0; }
    .code-box { background: rgba(246, 135, 179, 0.1); border: 1px solid rgba(246, 135, 179, 0.3); border-radius: 12px; padding: 24px; text-align: center; margin: 24px 0; }
    .code { font-size: 36px; font-weight: 700; letter-spacing: 8px; color: #f687b3; font-family: 'SF Mono', Monaco, monospace; }
    .tips { background: rgba(255,255,255,0.05); border-radius: 8px; padding: 16px; margin-top: 24px; }
    .tips-title { color: #f687b3; font-weight: 600; margin-bottom: 12px; font-size: 14px; }
    .tips ul { margin: 0; padding-left: 20px; color: #718096; font-size: 13px; line-height: 1.8; }
    .footer { padding: 24px 32px; text-align: center; border-top: 1px solid rgba(255,255,255,0.05); }
    .footer p { color: #4a5568; font-size: 12px; margin: 0; }
  </style>
</head>
<body>
  <div class="container">
    <div class="header">
      <h1>密码重置</h1>
    </div>
    <div class="content">
      <p class="greeting">尊敬的用户：</p>
      <p class="message">您正在进行密码重置操作，请输入以下验证码完成验证：</p>
      <div class="code-box">
        <span class="code">%s</span>
      </div>
      <div class="tips">
        <p class="tips-title">安全提示：</p>
        <ul>
          <li>验证码有效期为5分钟</li>
          <li>请勿将验证码泄露给他人</li>
          <li>如非本人操作，请立即修改密码</li>
        </ul>
      </div>
    </div>
    <div class="footer">
      <p>此邮件由系统自动发送，请勿回复</p>
    </div>
  </div>
</body>
</html>
`, code)
	}

	return s.sendEmail(toEmail, subject, bodyText)
}

// sendEmail sends an email using SMTP
func (s *Service) sendEmail(to, subject, body string) error {
	from := s.config.FromAddress
	if from == "" {
		from = s.config.Username
	}

	// Create email headers
	headers := make(map[string]string)
	headers["From"] = fmt.Sprintf("%s <%s>", s.config.FromName, from)
	headers["To"] = to
	headers["Subject"] = subject
	headers["MIME-Version"] = "1.0"
	headers["Content-Type"] = "text/html; charset=UTF-8"

	// Build message
	message := ""
	for k, v := range headers {
		message += fmt.Sprintf("%s: %s\r\n", k, v)
	}
	message += "\r\n" + body

	// SMTP authentication
	auth := smtp.PlainAuth("", s.config.Username, s.config.Password, s.config.SMTPHost)

	// Use SSL/TLS for port 465
	if s.config.UseSSL {
		return s.sendEmailWithSSL(to, from, message, auth)
	}

	// Use standard SMTP for other ports
	addr := fmt.Sprintf("%s:%s", s.config.SMTPHost, s.config.SMTPPort)
	return smtp.SendMail(addr, auth, from, []string{to}, []byte(message))
}

// sendEmailWithSSL sends email using SSL/TLS connection (for port 465)
func (s *Service) sendEmailWithSSL(to, from, message string, auth smtp.Auth) error {
	tlsConfig := &tls.Config{
		InsecureSkipVerify: true,
		ServerName:         s.config.SMTPHost,
	}

	addr := fmt.Sprintf("%s:%s", s.config.SMTPHost, s.config.SMTPPort)
	conn, err := tls.Dial("tcp", addr, tlsConfig)
	if err != nil {
		return fmt.Errorf("failed to connect to SMTP server: %w", err)
	}
	defer conn.Close()

	client, err := smtp.NewClient(conn, s.config.SMTPHost)
	if err != nil {
		return fmt.Errorf("failed to create SMTP client: %w", err)
	}
	defer client.Quit()

	if err = client.Auth(auth); err != nil {
		return fmt.Errorf("SMTP authentication failed: %w", err)
	}

	if err = client.Mail(from); err != nil {
		return fmt.Errorf("failed to set sender: %w", err)
	}

	if err = client.Rcpt(to); err != nil {
		return fmt.Errorf("failed to set recipient: %w", err)
	}

	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("failed to open data writer: %w", err)
	}

	_, err = w.Write([]byte(message))
	if err != nil {
		return fmt.Errorf("failed to write message: %w", err)
	}

	err = w.Close()
	if err != nil {
		return fmt.Errorf("failed to close data writer: %w", err)
	}

	return nil
}

// CleanExpiredCodes removes expired verification codes (should be called periodically)
func (s *Service) CleanExpiredCodes() {
	s.codesMutex.Lock()
	defer s.codesMutex.Unlock()

	now := time.Now()
	for email, vc := range s.codes {
		if now.After(vc.ExpiresAt) {
			delete(s.codes, email)
		}
	}
}
