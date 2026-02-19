package commands

import (
	"crypto/rand"
	"errors"
	"flag"
	"fmt"
	"math/big"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/lib/pq"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
	"goyco/internal/config"
	"goyco/internal/database"
	"goyco/internal/repositories"
	"goyco/internal/security"
	"goyco/internal/services"
)

func HandleUserCommand(cfg *config.Config, name string, args []string) error {
	fs := newFlagSet(name, printUserUsage)
	if err := parseCommand(fs, args, name); err != nil {
		if errors.Is(err, ErrHelpRequested) {
			return nil
		}
		return err
	}

	return withDatabase(cfg, func(db *gorm.DB) error {
		repo := repositories.NewUserRepository(db)
		refreshTokenRepo := repositories.NewRefreshTokenRepository(db)
		return runUserCommand(cfg, repo, refreshTokenRepo, fs.Args())
	})
}

func runUserCommand(cfg *config.Config, repo repositories.UserRepository, refreshTokenRepo repositories.RefreshTokenRepositoryInterface, args []string) error {
	if len(args) == 0 {
		printUserUsage()
		return errors.New("missing user subcommand")
	}

	switch args[0] {
	case "create":
		return userCreate(cfg, repo, args[1:])
	case "update":
		return userUpdate(cfg, repo, refreshTokenRepo, args[1:])
	case "delete":
		return userDelete(cfg, repo, args[1:])
	case "lock":
		return userLock(cfg, repo, args[1:])
	case "unlock":
		return userUnlock(cfg, repo, args[1:])
	case "list":
		return userList(repo, args[1:])
	case "help", "-h", "--help":
		printUserUsage()
		return nil
	default:
		printUserUsage()
		return fmt.Errorf("unknown user subcommand: %s", args[0])
	}
}

func printUserUsage() {
	fmt.Fprintln(os.Stderr, "User subcommands:")
	fmt.Fprintln(os.Stderr, "  create --username <name> --email <email> --password <password>")
	fmt.Fprintln(os.Stderr, "  update <id> [--username <name>] [--email <email>] [--password <password>] [--reset-password]")
	fmt.Fprintln(os.Stderr, "  delete <id> [--with-posts]")
	fmt.Fprintln(os.Stderr, "  lock <id>")
	fmt.Fprintln(os.Stderr, "  unlock <id>")
	fmt.Fprintln(os.Stderr, "  list [--limit <n>] [--offset <n>]")
}

func createSessionService(cfg *config.Config, userRepo repositories.UserRepository, refreshTokenRepo repositories.RefreshTokenRepositoryInterface) *services.SessionService {
	jwtService := services.NewJWTService(&cfg.JWT, userRepo, refreshTokenRepo)
	return services.NewSessionService(jwtService, userRepo)
}

func userCreate(cfg *config.Config, repo repositories.UserRepository, args []string) error {
	fs := flag.NewFlagSet("user create", flag.ContinueOnError)
	username := fs.String("username", "", "username")
	email := fs.String("email", "", "email")
	password := fs.String("password", "", "password")
	fs.SetOutput(os.Stderr)

	if err := fs.Parse(args); err != nil {
		return err
	}

	if *username == "" || *email == "" || *password == "" {
		fs.Usage()
		return errors.New("username, email, and password are required")
	}

	auditLogger, err := NewAuditLogger(cfg.LogDir)
	if err != nil {
		outputWarning("Could not initialize audit logging: %v", err)
		auditLogger = nil
	}

	sanitizer := security.NewInputSanitizer()

	sanitizedUsername, err := sanitizer.SanitizeUsernameCLI(*username)
	if err != nil {
		if auditLogger != nil {
			auditLogger.LogUserCreation(0, *username, *email, false, err)
		}
		return fmt.Errorf("username validation: %w", err)
	}

	sanitizedEmail, err := sanitizer.SanitizeEmailCLI(*email)
	if err != nil {
		if auditLogger != nil {
			auditLogger.LogUserCreation(0, sanitizedUsername, *email, false, err)
		}
		return fmt.Errorf("email validation: %w", err)
	}

	if err := sanitizer.SanitizePasswordCLI(*password); err != nil {
		if auditLogger != nil {
			auditLogger.LogUserCreation(0, sanitizedUsername, sanitizedEmail, false, err)
		}
		return fmt.Errorf("password validation: %w", err)
	}

	_, err = repo.GetByUsername(sanitizedUsername)
	if err == nil {
		return fmt.Errorf("username %s already exists", sanitizedUsername)
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return fmt.Errorf("check username: %w", err)
	}

	_, err = repo.GetByEmail(sanitizedEmail)
	if err == nil {
		return fmt.Errorf("email %s already exists", sanitizedEmail)
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return fmt.Errorf("check email: %w", err)
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(*password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}

	now := time.Now()
	user := &database.User{
		Username:        sanitizedUsername,
		Email:           sanitizedEmail,
		Password:        string(hashedPassword),
		EmailVerified:   true,
		EmailVerifiedAt: &now,
	}

	if err := repo.Create(user); err != nil {
		if auditLogger != nil {
			auditLogger.LogUserCreation(0, sanitizedUsername, sanitizedEmail, false, err)
		}
		return handleDatabaseConstraintError(err)
	}

	if auditLogger != nil {
		auditLogger.LogUserCreation(user.ID, user.Username, user.Email, true, nil)
	}

	if IsJSONOutput() {
		outputJSON(map[string]any{
			"action":   "user_created",
			"id":       user.ID,
			"username": user.Username,
			"email":    user.Email,
		})
	} else {
		fmt.Printf("User created: %s (%s)\n", user.Username, user.Email)
	}
	return nil
}

func userUpdate(cfg *config.Config, repo repositories.UserRepository, refreshTokenRepo repositories.RefreshTokenRepositoryInterface, args []string) error {
	if len(args) == 0 {
		return errors.New("user ID is required")
	}

	idStr := args[0]
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		return fmt.Errorf("invalid user ID: %s", idStr)
	}

	if id == 0 {
		return errors.New("user ID must be greater than 0")
	}

	fs := flag.NewFlagSet("user update", flag.ContinueOnError)
	username := fs.String("username", "", "new username")
	email := fs.String("email", "", "new email")
	password := fs.String("password", "", "new password")
	resetPassword := fs.Bool("reset-password", false, "reset password and send temporary password via email")
	fs.SetOutput(os.Stderr)

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage of user update:\n")
		fmt.Fprintf(os.Stderr, "  --email string\n")
		fmt.Fprintf(os.Stderr, "    	new email\n")
		fmt.Fprintf(os.Stderr, "  --password string\n")
		fmt.Fprintf(os.Stderr, "    	new password\n")
		fmt.Fprintf(os.Stderr, "  --reset-password\n")
		fmt.Fprintf(os.Stderr, "    	reset password and send temporary password via email\n")
		fmt.Fprintf(os.Stderr, "  --username string\n")
		fmt.Fprintf(os.Stderr, "    	new username\n")
	}

	if err := fs.Parse(args[1:]); err != nil {
		return err
	}

	if *username == "" && *email == "" && *password == "" && !*resetPassword {
		fs.Usage()
		return errors.New("no update options provided")
	}

	sanitizer := security.NewInputSanitizer()

	if *username != "" {
		sanitizedUsername, err := sanitizer.SanitizeUsernameCLI(*username)
		if err != nil {
			return fmt.Errorf("username validation: %w", err)
		}
		*username = sanitizedUsername
	}

	if *email != "" {
		sanitizedEmail, err := sanitizer.SanitizeEmailCLI(*email)
		if err != nil {
			return fmt.Errorf("email validation: %w", err)
		}
		*email = sanitizedEmail
	}

	if *password != "" {
		if err := sanitizer.SanitizePasswordCLI(*password); err != nil {
			return fmt.Errorf("password validation: %w", err)
		}
	}

	if *resetPassword {
		sessionService := createSessionService(cfg, repo, refreshTokenRepo)
		return resetUserPassword(cfg, repo, sessionService, uint(id))
	}

	user, err := repo.GetByID(uint(id))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("user %d not found", id)
		}
		return fmt.Errorf("fetch user: %w", err)
	}

	if *username != "" && *username != user.Username {
		if err := checkUsernameAvailable(repo, *username, uint(id)); err != nil {
			return err
		}
		user.Username = *username
	}

	if *email != "" && *email != user.Email {
		if err := checkEmailAvailable(repo, *email, uint(id)); err != nil {
			return err
		}
		user.Email = *email
	}

	if *password != "" {
		if len(*password) < 8 {
			return errors.New("password must be at least 8 characters")
		}
		hashedPassword, hashErr := bcrypt.GenerateFromPassword([]byte(*password), bcrypt.DefaultCost)
		if hashErr != nil {
			return fmt.Errorf("hash password: %w", hashErr)
		}
		user.Password = string(hashedPassword)

		sessionService := createSessionService(cfg, repo, refreshTokenRepo)
		if err := sessionService.InvalidateAllSessions(user.ID); err != nil {
			return fmt.Errorf("invalidate sessions: %w", err)
		}
	}

	if err := repo.Update(user); err != nil {
		return handleDatabaseConstraintError(err)
	}

	if IsJSONOutput() {
		outputJSON(map[string]any{
			"action":   "user_updated",
			"id":       user.ID,
			"username": user.Username,
			"email":    user.Email,
		})
	} else {
		fmt.Printf("User updated: %s (%s)\n", user.Username, user.Email)
	}
	return nil
}

func checkUsernameAvailable(repo repositories.UserRepository, username string, excludeID uint) error {
	existing, err := repo.GetByUsernameIncludingDeleted(username)
	if err == nil && existing.ID != excludeID {
		return fmt.Errorf("username %s is already taken", username)
	}
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return fmt.Errorf("check username availability: %w", err)
	}
	return nil
}

func checkEmailAvailable(repo repositories.UserRepository, email string, excludeID uint) error {
	existing, err := repo.GetByEmail(email)
	if err == nil && existing.ID != excludeID {
		return fmt.Errorf("email %s is already registered", email)
	}
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return fmt.Errorf("check email availability: %w", err)
	}
	return nil
}

func handleDatabaseConstraintError(err error) error {
	var pqErr *pq.Error
	if errors.As(err, &pqErr) && pqErr.Code == "23505" {
		if strings.Contains(pqErr.Constraint, "username") {
			return fmt.Errorf("username is already taken")
		}
		if strings.Contains(pqErr.Constraint, "email") {
			return fmt.Errorf("email is already registered")
		}
		return fmt.Errorf("data already exists (constraint violation)")
	}

	return fmt.Errorf("update user: %w", err)
}

func userDelete(cfg *config.Config, repo repositories.UserRepository, args []string) error {
	var userID string
	var flagArgs []string

	for _, arg := range args {
		if strings.HasPrefix(arg, "-") {
			flagArgs = append(flagArgs, arg)
		} else if userID == "" {
			userID = arg
		} else {
			flagArgs = append(flagArgs, arg)
		}
	}

	fs := flag.NewFlagSet("user delete", flag.ContinueOnError)
	deletePosts := fs.Bool("with-posts", false, "also delete user's posts (default: keep posts)")
	fs.SetOutput(os.Stderr)

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage of user delete:\n")
		fmt.Fprintf(os.Stderr, "  --with-posts\n")
		fmt.Fprintf(os.Stderr, "    	also delete user's posts (default: keep posts)\n")
	}

	if err := fs.Parse(flagArgs); err != nil {
		return err
	}

	if userID == "" {
		fs.Usage()
		return errors.New("user ID is required")
	}

	idStr := userID
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		return fmt.Errorf("invalid user ID: %s", idStr)
	}

	if id == 0 {
		return errors.New("user ID must be greater than 0")
	}

	user, err := repo.GetByID(uint(id))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			_, deletedErr := repo.GetByIDIncludingDeleted(uint(id))
			if deletedErr == nil {
				return fmt.Errorf("user with ID %d is already deleted", id)
			}
			return fmt.Errorf("user with ID %d not found", id)
		}
		return fmt.Errorf("get user: %w", err)
	}

	var deleteErr error
	var postsDeleted bool
	if *deletePosts {
		deleteErr = repo.HardDelete(uint(id))
		postsDeleted = true
	} else {
		deleteErr = repo.SoftDeleteWithPosts(uint(id))
		postsDeleted = false
	}

	if deleteErr != nil {
		return fmt.Errorf("delete user: %w", deleteErr)
	}

	emailSender := services.NewSMTPSenderWithTimeout(cfg.SMTP.Host, cfg.SMTP.Port, cfg.SMTP.Username, cfg.SMTP.Password, cfg.SMTP.From, cfg.SMTP.Timeout)
	subject, body := services.GenerateAdminAccountDeletionNotificationEmail(user.Username, cfg.App.AdminEmail, cfg.App.BaseURL, cfg.App.Title, *deletePosts)

	emailSent := true
	if err := emailSender.Send(user.Email, subject, body); err != nil {
		outputWarning("Could not send notification email to %s: %v", user.Email, err)
		emailSent = false
	}

	if IsJSONOutput() {
		outputJSON(map[string]any{
			"action":        "user_deleted",
			"id":            id,
			"username":      user.Username,
			"email":         user.Email,
			"posts_deleted": postsDeleted,
			"email_sent":    emailSent,
		})
	} else {
		if postsDeleted {
			fmt.Printf("User deleted: ID=%d (posts also deleted)\n", id)
		} else {
			fmt.Printf("User deleted: ID=%d (posts kept)\n", id)
		}
		if emailSent {
			fmt.Printf("Notification email sent to %s\n", user.Email)
		}
	}

	return nil
}

func userList(repo repositories.UserRepository, args []string) error {
	fs := flag.NewFlagSet("user list", flag.ContinueOnError)
	limit := fs.Int("limit", 0, "max number of users to list")
	offset := fs.Int("offset", 0, "number of users to skip")
	fs.SetOutput(os.Stderr)

	if err := fs.Parse(args); err != nil {
		return err
	}

	users, err := repo.GetAll(*limit, *offset)
	if err != nil {
		return fmt.Errorf("list users: %w", err)
	}

	if IsJSONOutput() {
		type userJSON struct {
			ID        uint   `json:"id"`
			Username  string `json:"username"`
			Email     string `json:"email"`
			Locked    bool   `json:"locked"`
			CreatedAt string `json:"created_at"`
		}
		usersJSON := make([]userJSON, len(users))
		for i, u := range users {
			usersJSON[i] = userJSON{
				ID:        u.ID,
				Username:  u.Username,
				Email:     u.Email,
				Locked:    u.Locked,
				CreatedAt: u.CreatedAt.Format("2006-01-02 15:04:05"),
			}
		}
		outputJSON(map[string]any{
			"users": usersJSON,
			"count": len(usersJSON),
		})
		return nil
	}

	if len(users) == 0 {
		fmt.Println("No users found")
		return nil
	}

	maxIDWidth := 2
	maxUsernameWidth := 8
	maxEmailWidth := 5
	maxLockedWidth := 6
	maxCreatedAtWidth := 10

	for _, u := range users {
		lockedStatus := "No"
		if u.Locked {
			lockedStatus = "Yes"
		}
		createdAtStr := u.CreatedAt.Format("2006-01-02 15:04:05")

		if len(fmt.Sprintf("%d", u.ID)) > maxIDWidth {
			maxIDWidth = len(fmt.Sprintf("%d", u.ID))
		}
		if len(u.Username) > maxUsernameWidth {
			maxUsernameWidth = len(u.Username)
		}
		if len(u.Email) > maxEmailWidth {
			maxEmailWidth = len(u.Email)
		}
		if len(lockedStatus) > maxLockedWidth {
			maxLockedWidth = len(lockedStatus)
		}
		if len(createdAtStr) > maxCreatedAtWidth {
			maxCreatedAtWidth = len(createdAtStr)
		}
	}

	fmt.Printf("%-*s %-*s %-*s %-*s %s\n",
		maxIDWidth, "ID",
		maxUsernameWidth, "Username",
		maxEmailWidth, "Email",
		maxLockedWidth, "Locked",
		"CreatedAt")

	for _, u := range users {
		lockedStatus := "No"
		if u.Locked {
			lockedStatus = "Yes"
		}
		createdAtStr := u.CreatedAt.Format("2006-01-02 15:04:05")

		fmt.Printf("%-*d %-*s %-*s %-*s %s\n",
			maxIDWidth, u.ID,
			maxUsernameWidth, u.Username,
			maxEmailWidth, u.Email,
			maxLockedWidth, lockedStatus,
			createdAtStr)
	}
	return nil
}

func userLock(cfg *config.Config, repo repositories.UserRepository, args []string) error {
	fs := flag.NewFlagSet("user lock", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	if err := fs.Parse(args); err != nil {
		return err
	}

	if fs.NArg() == 0 {
		fs.Usage()
		return errors.New("user ID is required")
	}

	idStr := fs.Arg(0)
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		return fmt.Errorf("invalid user ID: %s", idStr)
	}

	if id == 0 {
		return errors.New("user ID must be greater than 0")
	}

	user, err := repo.GetByID(uint(id))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("user with ID %d not found", id)
		}
		return fmt.Errorf("get user: %w", err)
	}

	if user.Locked {
		if IsJSONOutput() {
			outputJSON(map[string]any{
				"action":   "user_lock",
				"id":       id,
				"username": user.Username,
				"status":   "already_locked",
			})
		} else {
			fmt.Printf("User is already locked: %s\n", user.Username)
		}
		return nil
	}

	if err := repo.Lock(uint(id)); err != nil {
		return fmt.Errorf("lock user: %w", err)
	}

	emailSender := services.NewSMTPSenderWithTimeout(cfg.SMTP.Host, cfg.SMTP.Port, cfg.SMTP.Username, cfg.SMTP.Password, cfg.SMTP.From, cfg.SMTP.Timeout)
	subject, body := services.GenerateAccountLockNotificationEmail(user.Username, cfg.App.AdminEmail, cfg.App.Title)

	emailSent := true
	if err := emailSender.Send(user.Email, subject, body); err != nil {
		outputWarning("Could not send notification email to %s: %v", user.Email, err)
		emailSent = false
	}

	if IsJSONOutput() {
		outputJSON(map[string]any{
			"action":     "user_locked",
			"id":         id,
			"username":   user.Username,
			"email_sent": emailSent,
		})
	} else {
		fmt.Printf("User locked: %s\n", user.Username)
		if emailSent {
			fmt.Printf("Notification email sent to %s\n", user.Email)
		}
	}

	return nil
}

func userUnlock(cfg *config.Config, repo repositories.UserRepository, args []string) error {
	fs := flag.NewFlagSet("user unlock", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	if err := fs.Parse(args); err != nil {
		return err
	}

	if fs.NArg() == 0 {
		fs.Usage()
		return errors.New("user ID is required")
	}

	idStr := fs.Arg(0)
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		return fmt.Errorf("invalid user ID: %s", idStr)
	}

	if id == 0 {
		return errors.New("user ID must be greater than 0")
	}

	user, err := repo.GetByID(uint(id))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("user with ID %d not found", id)
		}
		return fmt.Errorf("get user: %w", err)
	}

	if !user.Locked {
		if IsJSONOutput() {
			outputJSON(map[string]any{
				"action":   "user_unlock",
				"id":       id,
				"username": user.Username,
				"status":   "already_unlocked",
			})
		} else {
			fmt.Printf("User is already unlocked: %s\n", user.Username)
		}
		return nil
	}

	if err := repo.Unlock(uint(id)); err != nil {
		return fmt.Errorf("unlock user: %w", err)
	}

	emailSender := services.NewSMTPSenderWithTimeout(cfg.SMTP.Host, cfg.SMTP.Port, cfg.SMTP.Username, cfg.SMTP.Password, cfg.SMTP.From, cfg.SMTP.Timeout)
	subject, body := services.GenerateAccountUnlockNotificationEmail(user.Username, cfg.App.AdminEmail, cfg.App.BaseURL, cfg.App.Title)

	emailSent := true
	if err := emailSender.Send(user.Email, subject, body); err != nil {
		outputWarning("Could not send notification email to %s: %v", user.Email, err)
		emailSent = false
	}

	if IsJSONOutput() {
		outputJSON(map[string]any{
			"action":     "user_unlocked",
			"id":         id,
			"username":   user.Username,
			"email_sent": emailSent,
		})
	} else {
		fmt.Printf("User unlocked: %s\n", user.Username)
		if emailSent {
			fmt.Printf("Notification email sent to %s\n", user.Email)
		}
	}

	return nil
}

func resetUserPassword(cfg *config.Config, repo repositories.UserRepository, sessionService *services.SessionService, userID uint) error {
	user, err := repo.GetByID(userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("user %d not found", userID)
		}
		return fmt.Errorf("fetch user: %w", err)
	}

	tempPassword, err := generateTemporaryPassword()
	if err != nil {
		return fmt.Errorf("generate temporary password: %w", err)
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(tempPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}

	user.Password = string(hashedPassword)
	if err := repo.Update(user); err != nil {
		return fmt.Errorf("update password: %w", err)
	}

	if err := sessionService.InvalidateAllSessions(userID); err != nil {
		return fmt.Errorf("invalidate sessions: %w", err)
	}

	emailSender := services.NewSMTPSenderWithTimeout(cfg.SMTP.Host, cfg.SMTP.Port, cfg.SMTP.Username, cfg.SMTP.Password, cfg.SMTP.From, cfg.SMTP.Timeout)
	subject := fmt.Sprintf("Password Reset - %s", cfg.App.Title)
	body := generatePasswordResetEmailBody(user.Username, tempPassword, cfg.App.BaseURL, cfg.App.AdminEmail, cfg.App.Title)

	if err := emailSender.Send(user.Email, subject, body); err != nil {
		return fmt.Errorf("send password reset email: %w", err)
	}

	if IsJSONOutput() {
		outputJSON(map[string]any{
			"action":   "password_reset",
			"id":       userID,
			"username": user.Username,
			"email":    user.Email,
			"message":  "Temporary password sent. User must change password on next login.",
		})
	} else {
		fmt.Printf("Password reset for user %s: Temporary password sent to %s\n", user.Username, user.Email)
		fmt.Printf("⚠️ User must change this password on next login!\n")
	}

	return nil
}

func generateTemporaryPassword() (string, error) {
	const (
		length = 16
		chars  = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789!@#$%^&*"
	)

	password := make([]byte, length)
	for i := range password {
		num, err := rand.Int(rand.Reader, big.NewInt(int64(len(chars))))
		if err != nil {
			return "", err
		}
		password[i] = chars[num.Int64()]
	}

	passwordStr := string(password)

	hasUpper := false
	hasLower := false
	hasDigit := false
	hasSpecial := false

	for _, char := range passwordStr {
		switch {
		case char >= 'A' && char <= 'Z':
			hasUpper = true
		case char >= 'a' && char <= 'z':
			hasLower = true
		case char >= '0' && char <= '9':
			hasDigit = true
		case strings.ContainsRune("!@#$%^&*", char):
			hasSpecial = true
		}
	}

	passwordBytes := []byte(passwordStr)

	if !hasUpper {
		passwordBytes[0] = 'A'
	}
	if !hasLower {
		passwordBytes[1] = 'a'
	}
	if !hasDigit {
		passwordBytes[2] = '1'
	}
	if !hasSpecial {
		passwordBytes[3] = '!'
	}

	hasUpper = false
	hasLower = false
	hasDigit = false
	hasSpecial = false

	for _, char := range passwordBytes {
		switch {
		case char >= 'A' && char <= 'Z':
			hasUpper = true
		case char >= 'a' && char <= 'z':
			hasLower = true
		case char >= '0' && char <= '9':
			hasDigit = true
		case char == '!' || char == '@' || char == '#' || char == '$' || char == '%' || char == '^' || char == '&' || char == '*':
			hasSpecial = true
		}
	}

	if !hasUpper {
		passwordBytes[4] = 'A'
	}
	if !hasLower {
		passwordBytes[5] = 'a'
	}
	if !hasDigit {
		passwordBytes[6] = '1'
	}
	if !hasSpecial {
		passwordBytes[7] = '!'
	}

	return string(passwordBytes), nil
}

func generatePasswordResetEmailBody(username, tempPassword, baseURL, adminEmail, siteTitle string) string {
	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Password Reset - %s</title>
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
        .password-box {
            background: #f7fafc;
            border: 2px solid #e2e8f0;
            border-radius: 8px;
            padding: 20px;
            margin: 20px 0;
            text-align: center;
        }
        .password-label {
            font-size: 14px;
            color: #718096;
            margin-bottom: 10px;
        }
        .password-value {
            font-size: 24px;
            font-weight: 700;
            color: #2d3748;
            font-family: 'Courier New', monospace;
            letter-spacing: 2px;
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
        .security-notice {
            background: #fef5e7;
            border: 1px solid #f6ad55;
            border-radius: 8px;
            padding: 20px;
            margin: 20px 0;
        }
        .security-title {
            font-weight: 600;
            color: #c05621;
            margin-bottom: 10px;
        }
        .security-list {
            margin: 0;
            padding-left: 20px;
            color: #744210;
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
            <h1 class="title">Password Reset - Temporary Password</h1>
        </div>
        
        <div class="content">
            <div class="greeting">Hello %s,</div>
            <div class="message">Your password has been reset by an administrator.

A temporary password has been generated for your account.</div>
            
            <div class="password-box">
                <div class="password-label">Your temporary password is:</div>
                <div class="password-value">%s</div>
            </div>
            
            <div class="security-notice">
                <div class="security-title">IMPORTANT SECURITY NOTICE:</div>
                <ul class="security-list">
                    <li>You MUST change this password immediately after logging in</li>
                    <li>This temporary password will expire and should not be used long-term</li>
                    <li>Do not share this password with anyone</li>
                    <li>If you did not request this password reset, contact support immediately</li>
                </ul>
            </div>
            
            <div style="text-align: center;">
                <a href="%s/login" class="action-button">Login to %s</a>
            </div>
            
            <div class="message">To change your password:
1. Log in to your account using the temporary password above
2. Go to your account settings
3. Change your password to a new, secure password</div>
        </div>
        
        <div class="footer">
            If you have any questions or concerns, please <a href="mailto:%s" class="link">contact our support team</a>.<br>
            Best regards,<br>
            The %s Team
        </div>
        
        <div class="powered-by" style="text-align: center; margin-top: 30px; padding-top: 20px; border-top: 1px solid #e2e8f0; font-size: 12px; color: #718096;">
            Powered with ❤️ by <a href="https://goyco" style="color: #0fb9b1; text-decoration: none;">Goyco</a>
        </div>
    </div>
</body>
</html>`, siteTitle, siteTitle, username, tempPassword, baseURL, siteTitle, adminEmail, siteTitle)
}
