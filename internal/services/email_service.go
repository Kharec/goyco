package services

import (
	"fmt"
	"html"
	"net/url"
	"strings"

	"goyco/internal/config"
	"goyco/internal/database"
)

type EmailService struct {
	EmailSender EmailSender
	baseURL     string
	config      *config.Config
}

func NewEmailService(cfg *config.Config, sender EmailSender) (*EmailService, error) {
	baseURL := strings.TrimRight(strings.TrimSpace(cfg.App.BaseURL), "/")
	if baseURL == "" {
		return nil, fmt.Errorf("APP_BASE_URL is required and must be externally reachable")
	}

	return &EmailService{
		EmailSender: sender,
		baseURL:     baseURL,
		config:      cfg,
	}, nil
}

func (s *EmailService) SendVerificationEmail(user *database.User, token string) error {
	if s.EmailSender == nil {
		return ErrEmailSenderUnavailable
	}

	verificationURL := fmt.Sprintf("%s/confirm?token=%s", s.baseURL, url.QueryEscape(token))
	subject := fmt.Sprintf("🎉 Welcome to %s! Confirm your email address", s.config.App.Title)
	body := s.GenerateVerificationEmailBody(user.Username, verificationURL)

	if err := s.EmailSender.Send(user.Email, subject, body); err != nil {
		return fmt.Errorf("send verification email: %w", err)
	}

	return nil
}

func (s *EmailService) SendEmailChangeVerificationEmail(user *database.User, token string) error {
	if s.EmailSender == nil {
		return ErrEmailSenderUnavailable
	}

	verificationURL := fmt.Sprintf("%s/confirm?token=%s", s.baseURL, url.QueryEscape(token))
	subject := "📧 Confirm your new email address"
	body := s.GenerateEmailChangeVerificationEmailBody(user.Username, verificationURL)

	if err := s.EmailSender.Send(user.Email, subject, body); err != nil {
		return fmt.Errorf("send email change verification email: %w", err)
	}

	return nil
}

func (s *EmailService) SendResendVerificationEmail(user *database.User, token string) error {
	if s.EmailSender == nil {
		return ErrEmailSenderUnavailable
	}

	verificationURL := fmt.Sprintf("%s/confirm?token=%s", s.baseURL, url.QueryEscape(token))
	subject := fmt.Sprintf("🔄 Resend: Confirm your %s account", s.config.App.Title)
	body := s.GenerateResendVerificationEmailBody(user.Username, verificationURL)

	if err := s.EmailSender.Send(user.Email, subject, body); err != nil {
		return fmt.Errorf("send verification email: %w", err)
	}

	return nil
}

func (s *EmailService) SendPasswordResetEmail(user *database.User, token string) error {
	if s.EmailSender == nil {
		return ErrEmailSenderUnavailable
	}

	resetURL := fmt.Sprintf("%s/reset-password?token=%s", s.baseURL, url.QueryEscape(token))
	subject := fmt.Sprintf("Reset your %s password", s.config.App.Title)
	body := s.GeneratePasswordResetEmailBody(user.Username, resetURL)

	if err := s.EmailSender.Send(user.Email, subject, body); err != nil {
		return fmt.Errorf("send password reset email: %w", err)
	}

	return nil
}

func (s *EmailService) SendAccountDeletionEmail(user *database.User, token string) error {
	if s.EmailSender == nil {
		return ErrEmailSenderUnavailable
	}

	confirmationURL := fmt.Sprintf("%s/settings/delete/confirm?token=%s", s.baseURL, url.QueryEscape(token))
	subject := "Confirm Account Deletion"
	body := s.GenerateAccountDeletionEmailBody(user.Username, confirmationURL)

	if err := s.EmailSender.Send(user.Email, subject, body); err != nil {
		return fmt.Errorf("send account deletion email: %w", err)
	}

	return nil
}

func (s *EmailService) SendAccountDeletionNotificationEmail(user *database.User, deletedPosts bool) error {
	if s.EmailSender == nil {
		return ErrEmailSenderUnavailable
	}

	subject, body := GenerateAccountDeletionNotificationEmail(user.Username, s.config.App.AdminEmail, s.baseURL, s.config.App.Title, deletedPosts)

	if err := s.EmailSender.Send(user.Email, subject, body); err != nil {
		return fmt.Errorf("send account deletion notification email: %w", err)
	}

	return nil
}

func (s *EmailService) SendAccountLockNotificationEmail(user *database.User) error {
	if s.EmailSender == nil {
		return ErrEmailSenderUnavailable
	}

	subject, body := GenerateAccountLockNotificationEmail(user.Username, s.config.App.AdminEmail, s.config.App.Title)

	if err := s.EmailSender.Send(user.Email, subject, body); err != nil {
		return fmt.Errorf("send account lock notification email: %w", err)
	}

	return nil
}

func (s *EmailService) SendAccountUnlockNotificationEmail(user *database.User) error {
	if s.EmailSender == nil {
		return ErrEmailSenderUnavailable
	}

	subject, body := GenerateAccountUnlockNotificationEmail(user.Username, s.config.App.AdminEmail, s.baseURL, s.config.App.Title)

	if err := s.EmailSender.Send(user.Email, subject, body); err != nil {
		return fmt.Errorf("send account unlock notification email: %w", err)
	}

	return nil
}

func (s *EmailService) GenerateVerificationEmailBody(username, verificationURL string) string {
	safeUsername := html.EscapeString(username)
	safeURL := html.EscapeString(verificationURL)
	siteTitle := s.config.App.Title

	return s.generateStyledEmailBody(siteTitle, "🎉 Welcome! Confirm your email address",
		fmt.Sprintf("Hello %s,", safeUsername),
		fmt.Sprintf("Welcome to %s! Please confirm your email address by clicking the link below:", siteTitle),
		safeURL, "Confirm Email Address",
		"If the link doesn't work, you can copy and paste it into your browser.\n\nOnce confirmed, you'll be able to:\n- Create and share posts with the community\n- Vote on content you find interesting\n- Connect with other members\n- Customize your profile and preferences\n\nIf you didn't create this account, you can safely ignore this email.")
}

func (s *EmailService) GenerateEmailChangeVerificationEmailBody(username, verificationURL string) string {
	safeUsername := html.EscapeString(username)
	safeURL := html.EscapeString(verificationURL)
	siteTitle := s.config.App.Title

	return s.generateStyledEmailBody(siteTitle, "📧 Confirm your new email address",
		fmt.Sprintf("Hello %s,", safeUsername),
		"You've requested to change your email address. To complete this change, please confirm your new email address by clicking the link below:\n\nIf the link doesn't work, you can copy and paste it into your browser.\n\nOnce confirmed, your new email address will be active and you'll need to use it for future logins.\n\nIf you didn't request this email change, please contact our support team immediately.",
		safeURL, "Confirm New Email Address",
		"")
}

func (s *EmailService) GenerateResendVerificationEmailBody(username, verificationURL string) string {
	safeUsername := html.EscapeString(username)
	safeURL := html.EscapeString(verificationURL)
	siteTitle := s.config.App.Title

	return s.generateStyledEmailBody(siteTitle, fmt.Sprintf("🔄 Resend: Confirm your %s account", siteTitle),
		fmt.Sprintf("Hello %s,", safeUsername),
		"We've sent you a new verification link.\n\nPlease confirm your email address by clicking the link below:\n\nIf you're having trouble with the verification link:\n- Check your spam/junk folder\n- Make sure you're clicking the most recent email\n- Try copying the link and pasting it in your browser\n- Contact support if the problem persists\n\nIf you didn't request this email, you can safely ignore this message.",
		safeURL, "Confirm Email Address",
		"")
}

func (s *EmailService) GeneratePasswordResetEmailBody(username, resetURL string) string {
	safeUsername := html.EscapeString(username)
	safeURL := html.EscapeString(resetURL)
	siteTitle := s.config.App.Title

	return s.generateStyledEmailBody(siteTitle, fmt.Sprintf("Reset your %s password", siteTitle),
		fmt.Sprintf("Hello %s,", safeUsername),
		"We received a request to reset your password. To reset it, click the link below:",
		safeURL, "Reset Password",
		"This link will expire in 24 hours. If you didn't make this request, you can safely ignore this message.")
}

func (s *EmailService) GenerateAccountDeletionEmailBody(username, confirmationURL string) string {
	safeUsername := html.EscapeString(username)
	safeURL := html.EscapeString(confirmationURL)
	siteTitle := s.config.App.Title

	return s.generateStyledEmailBody(siteTitle, "Confirm Account Deletion",
		fmt.Sprintf("Hello %s,", safeUsername),
		fmt.Sprintf("We received a request to delete your %s account.\n\nTo confirm, click the link below:", siteTitle),
		safeURL, "Confirm Account Deletion",
		"This link will expire in 24 hours.\n\nIf you didn't make this request, you can safely ignore this message.")
}

func (s *EmailService) generateStyledEmailBody(siteTitle, emailTitle, greeting, message, actionURL, actionText, footer string) string {
	safeAdminEmail := html.EscapeString(s.config.App.AdminEmail)

	var buttonHTML string
	if actionURL != "" && actionText != "" {
		buttonHTML = fmt.Sprintf(`<div style="text-align: center;">
                <a href="%s" class="action-button">%s</a>
            </div>`, actionURL, actionText)
	}

	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>%s</title>
    <style>
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Oxygen, Ubuntu, Cantarell, sans-serif;
            line-height: 1.6;
            color: #333;
            max-width: 600px;
            margin: 0 auto;
            padding: 20px;
            background-color: #f8fafc;
        }
        .email-container {
            background: white;
            border-radius: 12px;
            padding: 40px;
            box-shadow: 0 4px 6px rgba(0, 0, 0, 0.05);
            border: 1px solid #e2e8f0;
        }
        .header {
            text-align: center;
            margin-bottom: 30px;
        }
        .logo {
            font-size: 28px;
            font-weight: 700;
            color: #0fb9b1;
            margin-bottom: 10px;
        }
        .title {
            font-size: 24px;
            font-weight: 600;
            color: #1a202c;
            margin: 0;
        }
        .content {
            margin-bottom: 30px;
        }
        .greeting {
            font-size: 16px;
            margin-bottom: 20px;
            color: #2d3748;
        }
        .message {
            font-size: 16px;
            margin-bottom: 30px;
            color: #4a5568;
            white-space: pre-line;
        }
        .action-button {
            display: inline-block;
            background: #0fb9b1;
            color: white;
            text-decoration: none;
            padding: 12px 24px;
            border-radius: 8px;
            font-weight: 600;
            font-size: 16px;
            text-align: center;
            margin: 20px 0;
            transition: background-color 0.2s;
        }
        .action-button:hover {
            background: #0ea5a0;
        }
        .footer {
            font-size: 14px;
            color: #718096;
            margin-top: 30px;
            padding-top: 20px;
            border-top: 1px solid #e2e8f0;
            white-space: pre-line;
        }
        .link {
            color: #0fb9b1;
            text-decoration: none;
        }
        .link:hover {
            text-decoration: underline;
        }
        @media (max-width: 600px) {
            body {
                padding: 10px;
            }
            .email-container {
                padding: 20px;
            }
            .title {
                font-size: 20px;
            }
        }
    </style>
</head>
<body>
    <div class="email-container">
        <div class="header">
            <div class="logo">%s</div>
            <h1 class="title">%s</h1>
        </div>
        
        <div class="content">
            <div class="greeting">%s</div>
            <div class="message">%s</div>
            %s
        </div>
        
        <div class="footer">
            %s

            If you have any questions or concerns, please <a href="mailto:%s" class="link">contact our support team</a>.<br>
            Best regards,<br>
            The %s Team
        </div>
        
        <div class="powered-by" style="text-align: center; margin-top: 30px; padding-top: 20px; border-top: 1px solid #e2e8f0; font-size: 12px; color: #718096;">
            Powered with ❤️ by <a href="https://goyco" style="color: #0fb9b1; text-decoration: none;">Goyco</a>
        </div>
    </div>
</body>
</html>`, emailTitle, siteTitle, emailTitle, greeting, message, buttonHTML, footer, safeAdminEmail, siteTitle)
}

func GenerateAccountDeletionNotificationEmail(username, adminEmail, baseURL, siteTitle string, deletedPosts bool) (string, string) {
	subject := "Account Deleted"
	safeUsername := html.EscapeString(username)
	safeAdminEmail := html.EscapeString(adminEmail)
	var body string
	if deletedPosts {
		body = fmt.Sprintf(`Your %s account has been permanently deleted.

All your posts have also been removed.

If you did not request this change, please <a href="mailto:%s" class="link">contact us</a>.

Thank you for being part of the community.`, siteTitle, safeAdminEmail)
	} else {
		body = fmt.Sprintf(`Your %s account has been permanently deleted.

Your posts have been preserved and are now anonymous.

If you did not request this change, please <a href="mailto:%s" class="link">contact us</a>.

Thank you for being part of the community.`, siteTitle, safeAdminEmail)
	}

	mainPageURL := strings.TrimRight(baseURL, "/")
	safeMainPageURL := html.EscapeString(mainPageURL)

	styledBody := generateStyledEmailBodyStatic(siteTitle, "Account Deleted",
		fmt.Sprintf("Hello %s,", safeUsername),
		body,
		safeMainPageURL, fmt.Sprintf("Visit %s", siteTitle),
		"", adminEmail)

	return subject, styledBody
}

func GenerateAdminAccountDeletionNotificationEmail(username, adminEmail, baseURL, siteTitle string, deletedPosts bool) (string, string) {
	subject := "Account Deleted by Administrator"
	safeUsername := html.EscapeString(username)
	safeAdminEmail := html.EscapeString(adminEmail)

	var message string
	if deletedPosts {
		message = fmt.Sprintf("Your %s account has been permanently deleted by an administrator.\n\nAll your posts have also been removed.\n\nIf you did not request this change, please <a href=\"mailto:%s\" class=\"link\">contact us</a>.\n\nThank you for being part of the community.", siteTitle, safeAdminEmail)
	} else {
		message = fmt.Sprintf("Your %s account has been permanently deleted by an administrator.\n\nYour posts have been preserved and are now anonymous.\n\nIf you did not request this change, please <a href=\"mailto:%s\" class=\"link\">contact us</a>.\n\nThank you for being part of the community.", siteTitle, safeAdminEmail)
	}

	mainPageURL := strings.TrimRight(baseURL, "/")
	safeMainPageURL := html.EscapeString(mainPageURL)

	body := generateStyledEmailBodyStatic(siteTitle, "Account Deleted by Administrator",
		fmt.Sprintf("Hello %s,", safeUsername),
		message,
		safeMainPageURL, fmt.Sprintf("Visit %s", siteTitle),
		"", adminEmail)

	return subject, body
}

func GenerateAccountLockNotificationEmail(username, adminEmail, siteTitle string) (string, string) {
	subject := "Account Locked"
	safeUsername := html.EscapeString(username)
	safeAdminEmail := html.EscapeString(adminEmail)
	message := fmt.Sprintf("Your %s account has been locked by an administrator.\n\nYou will not be able to log in or access your account until it is unlocked.\n\nIf you believe this is an error or need to discuss your account status, please <a href=\"mailto:%s\" class=\"link\">contact us</a>.\n\nThis action was taken to protect the security and integrity of our platform.", siteTitle, safeAdminEmail)

	body := generateStyledEmailBodyStatic(siteTitle, "Account Locked",
		fmt.Sprintf("Hello %s,", safeUsername),
		message,
		"", "",
		"", adminEmail)

	return subject, body
}

func GenerateAccountUnlockNotificationEmail(username, adminEmail, baseURL, siteTitle string) (string, string) {
	subject := "Account Unlocked"
	safeUsername := html.EscapeString(username)
	message := fmt.Sprintf("Your %s account has been unlocked by an administrator.\n\nYou can now log in and access all your account features normally.\n\nAll your previous data and settings have been preserved.\n\nWelcome back!", siteTitle)

	loginURL := fmt.Sprintf("%s/login", strings.TrimRight(baseURL, "/"))
	safeLoginURL := html.EscapeString(loginURL)
	body := generateStyledEmailBodyStatic(siteTitle, "Account Unlocked",
		fmt.Sprintf("Hello %s,", safeUsername),
		message,
		safeLoginURL, fmt.Sprintf("Login to %s", siteTitle),
		"", adminEmail)

	return subject, body
}

func generateStyledEmailBodyStatic(siteTitle, emailTitle, greeting, message, actionURL, actionText, footer, adminEmail string) string {
	safeAdminEmail := html.EscapeString(adminEmail)

	var buttonHTML string
	if actionURL != "" && actionText != "" {
		buttonHTML = fmt.Sprintf(`<div style="text-align: center;">
                <a href="%s" class="action-button">%s</a>
            </div>`, actionURL, actionText)
	}

	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>%s</title>
    <style>
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Oxygen, Ubuntu, Cantarell, sans-serif;
            line-height: 1.6;
            color: #333;
            max-width: 600px;
            margin: 0 auto;
            padding: 20px;
            background-color: #f8fafc;
        }
        .email-container {
            background: white;
            border-radius: 12px;
            padding: 40px;
            box-shadow: 0 4px 6px rgba(0, 0, 0, 0.05);
            border: 1px solid #e2e8f0;
        }
        .header {
            text-align: center;
            margin-bottom: 30px;
        }
        .logo {
            font-size: 28px;
            font-weight: 700;
            color: #0fb9b1;
            margin-bottom: 10px;
        }
        .title {
            font-size: 24px;
            font-weight: 600;
            color: #1a202c;
            margin: 0;
        }
        .content {
            margin-bottom: 30px;
        }
        .greeting {
            font-size: 16px;
            margin-bottom: 20px;
            color: #2d3748;
        }
        .message {
            font-size: 16px;
            margin-bottom: 30px;
            color: #4a5568;
            white-space: pre-line;
        }
        .action-button {
            display: inline-block;
            background: #0fb9b1;
            color: white;
            text-decoration: none;
            padding: 12px 24px;
            border-radius: 8px;
            font-weight: 600;
            font-size: 16px;
            text-align: center;
            margin: 20px 0;
            transition: background-color 0.2s;
        }
        .action-button:hover {
            background: #0ea5a0;
        }
        .footer {
            font-size: 14px;
            color: #718096;
            margin-top: 30px;
            padding-top: 20px;
            border-top: 1px solid #e2e8f0;
            white-space: pre-line;
        }
        .link {
            color: #0fb9b1;
            text-decoration: none;
        }
        .link:hover {
            text-decoration: underline;
        }
        @media (max-width: 600px) {
            body {
                padding: 10px;
            }
            .email-container {
                padding: 20px;
            }
            .title {
                font-size: 20px;
            }
        }
    </style>
</head>
<body>
    <div class="email-container">
        <div class="header">
            <div class="logo">%s</div>
            <h1 class="title">%s</h1>
        </div>
        
        <div class="content">
            <div class="greeting">%s</div>
            <div class="message">%s</div>
            %s
        </div>
        
        <div class="footer">
            %s

            If you have any questions or concerns, please <a href="mailto:%s" class="link">contact our support team</a>.<br>
            Best regards,<br>
            The %s Team
        </div>
        
        <div class="powered-by" style="text-align: center; margin-top: 30px; padding-top: 20px; border-top: 1px solid #e2e8f0; font-size: 12px; color: #718096;">
            Powered with ❤️ by <a href="https://goyco" style="color: #0fb9b1; text-decoration: none;">Goyco</a>
        </div>
    </div>
</body>
</html>`, emailTitle, siteTitle, emailTitle, greeting, message, buttonHTML, footer, safeAdminEmail, siteTitle)
}
