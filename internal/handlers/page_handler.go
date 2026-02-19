package handlers

import (
	"context"
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"goyco/internal/config"
	"goyco/internal/database"
	"goyco/internal/middleware"
	"goyco/internal/repositories"
	"goyco/internal/services"
	"goyco/internal/validation"

	"github.com/go-chi/chi/v5"
)

type PageHandler struct {
	templatesDir string
	authService  AuthServiceInterface
	postRepo     repositories.PostRepository
	voteService  *services.VoteService
	userRepo     repositories.UserRepository
	titleFetcher services.TitleFetcher
	config       *config.Config
	postQueries  *services.PostQueries

	funcMap   template.FuncMap
	mu        sync.RWMutex
	templates map[string]*template.Template
}

type PageData struct {
	Title               string
	SiteTitle           string
	User                *database.User
	Posts               []database.Post
	PostsSort           string
	PostsSortTopURL     string
	PostsSortNewURL     string
	CurrentPath         string
	Post                *database.Post
	Errors              []string
	Flash               string
	FormValues          map[string]string
	FormErrors          map[string][]string
	CurrentVote         database.VoteType
	UpVotes             int
	DownVotes           int
	CSRFToken           string
	CSPNonce            string
	Score               int
	ShowLoginLinks      bool
	VerificationSuccess bool
	SearchQuery         string
	Token               string
	HasPosts            bool
	PostCount           int64
}

func (d *PageData) setFormError(field, message string) {
	if d.FormErrors == nil {
		d.FormErrors = make(map[string][]string)
	}
	d.FormErrors[field] = []string{message}
}

func (h *PageHandler) newPageData(title string) *PageData {
	return &PageData{
		Title:     title,
		SiteTitle: h.config.App.Title,
	}
}

func NewPageHandler(templatesDir string, authService AuthServiceInterface, postRepo repositories.PostRepository, voteService *services.VoteService, userRepo repositories.UserRepository, titleFetcher services.TitleFetcher, config *config.Config) (*PageHandler, error) {
	if templatesDir == "" {
		templatesDir = "internal/templates"
	}

	handler := &PageHandler{
		templatesDir: templatesDir,
		authService:  authService,
		postRepo:     postRepo,
		voteService:  voteService,
		userRepo:     userRepo,
		titleFetcher: titleFetcher,
		config:       config,
		postQueries:  services.NewPostQueries(postRepo, voteService),
		funcMap: template.FuncMap{
			"formatTime": func(t time.Time) string {
				if t.IsZero() {
					return ""
				}
				return t.Format("02 Jan 2006 15:04")
			},
			"truncate": func(s string, length int) string {
				if len(s) <= length {
					return s
				}
				if length <= 3 {
					return s[:length]
				}
				return s[:length-3] + "..."
			},
			"substr": func(s string, start, length int) string {
				if start >= len(s) {
					return ""
				}
				end := min(start+length, len(s))
				return s[start:end]
			},
			"upper": strings.ToUpper,
		},
		templates: make(map[string]*template.Template),
	}

	if err := handler.reloadTemplates(); err != nil {
		return nil, err
	}

	return handler, nil
}

func (h *PageHandler) Home(w http.ResponseWriter, r *http.Request) {
	user := h.currentUser(r)

	sortParam := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("sort")))
	postsSort := "top"

	ctx := services.VoteContext{
		UserID:    0,
		IPAddress: GetClientIP(r),
		UserAgent: r.UserAgent(),
	}

	if user != nil {
		ctx.UserID = user.ID
	}

	var (
		posts []database.Post
		err   error
	)

	switch sortParam {
	case "new", "newest", "latest":
		postsSort = "new"
		posts, err = h.postQueries.GetNewest(50, ctx)
	default:
		posts, err = h.postQueries.GetTop(50, ctx)
	}
	if err != nil {
		h.renderError(w, r, http.StatusInternalServerError, "Unable to load posts")
		return
	}

	currentPath := strings.TrimSpace(r.URL.RequestURI())
	if currentPath == "" {
		currentPath = "/"
	}

	token, err := h.generateCSRFToken(w, r)
	if err != nil {
		h.renderError(w, r, http.StatusInternalServerError, "Unable to generate security token")
		return
	}
	csrfToken := token

	data := h.newPageData(h.config.App.Title)
	data.User = user
	data.Posts = posts
	data.PostsSort = postsSort
	data.PostsSortTopURL = "/"
	data.PostsSortNewURL = "/?sort=new"
	data.CurrentPath = currentPath
	data.SearchQuery = ""
	data.CSRFToken = csrfToken

	h.render(w, r, "home.gohtml", data)
}

func (h *PageHandler) Search(w http.ResponseWriter, r *http.Request) {
	user := h.currentUser(r)
	query := strings.TrimSpace(r.URL.Query().Get("q"))

	ctx := services.VoteContext{
		UserID:    0,
		IPAddress: GetClientIP(r),
		UserAgent: r.UserAgent(),
	}

	if user != nil {
		ctx.UserID = user.ID
	}

	var posts []database.Post
	var err error

	if query != "" {
		opts := services.QueryOptions{
			Limit:  50,
			Offset: 0,
		}
		posts, err = h.postQueries.GetSearch(query, opts, ctx)
		if err != nil {
			h.renderError(w, r, http.StatusInternalServerError, "Unable to search posts")
			return
		}
	}

	currentPath := strings.TrimSpace(r.URL.RequestURI())
	if currentPath == "" {
		currentPath = "/search"
	}

	token, err := h.generateCSRFToken(w, r)
	if err != nil {
		h.renderError(w, r, http.StatusInternalServerError, "Unable to generate security token")
		return
	}
	csrfToken := token

	data := h.newPageData("Search results")
	data.User = user
	data.Posts = posts
	data.SearchQuery = query
	data.CurrentPath = currentPath
	data.CSRFToken = csrfToken

	h.render(w, r, "search.gohtml", data)
}

func (h *PageHandler) ShowPost(w http.ResponseWriter, r *http.Request) {
	user := h.currentUser(r)
	postIDStr := chi.URLParam(r, "id")

	postID, err := strconv.Atoi(postIDStr)
	if err != nil || postID <= 0 {
		h.renderError(w, r, http.StatusBadRequest, "Invalid post identifier")
		return
	}

	ctx := services.VoteContext{
		UserID:    0,
		IPAddress: GetClientIP(r),
		UserAgent: r.UserAgent(),
	}

	if user != nil {
		ctx.UserID = user.ID
	}

	post, err := h.postQueries.GetByID(uint(postID), ctx)
	if err != nil {
		h.renderError(w, r, http.StatusNotFound, "Post not found")
		return
	}

	data := h.newPageData(post.Title)
	data.User = user
	data.Post = post
	data.UpVotes = post.UpVotes
	data.DownVotes = post.DownVotes
	data.Score = post.Score

	if post.CurrentVote != "" {
		data.CurrentVote = post.CurrentVote
	}

	token, err := h.generateCSRFToken(w, r)
	if err != nil {
		h.renderError(w, r, http.StatusInternalServerError, "Unable to generate security token")
		return
	}
	data.CSRFToken = token

	h.render(w, r, "post.gohtml", data)
}

func (h *PageHandler) NewPostForm(w http.ResponseWriter, r *http.Request) {
	user := h.currentUser(r)
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	data := h.newPageData("Share a link")
	data.User = user
	data.FormValues = map[string]string{}

	token, err := h.generateCSRFToken(w, r)
	if err != nil {
		h.renderError(w, r, http.StatusInternalServerError, "Unable to generate security token")
		return
	}
	data.CSRFToken = token

	h.render(w, r, "new_post.gohtml", data)
}

func (h *PageHandler) CreatePost(w http.ResponseWriter, r *http.Request) {
	user := h.currentUserWithLockCheck(w, r)
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	if err := r.ParseForm(); err != nil {
		h.renderError(w, r, http.StatusBadRequest, "Invalid form submission")
		return
	}

	title := strings.TrimSpace(r.FormValue("title"))
	url := strings.TrimSpace(r.FormValue("url"))
	content := strings.TrimSpace(r.FormValue("content"))

	var errorsList []string
	if url == "" {
		errorsList = append(errorsList, "URL is required")
	}

	if title == "" && url != "" && h.titleFetcher != nil {
		titleCtx, cancel := context.WithTimeout(r.Context(), 7*time.Second)
		defer cancel()

		fetchedTitle, err := h.titleFetcher.FetchTitle(titleCtx, url)
		if err != nil {
			switch {
			case errors.Is(err, services.ErrUnsupportedScheme):
				errorsList = append(errorsList, "Only HTTP and HTTPS URLs are supported")
			case errors.Is(err, services.ErrTitleNotFound):
				errorsList = append(errorsList, "Title could not be extracted from the provided URL")
			default:
				errorsList = append(errorsList, "Failed to fetch title from URL")
			}
		} else {
			title = fetchedTitle
		}
	}

	if title == "" {
		errorsList = append(errorsList, "Title is required")
	}

	if len(errorsList) > 0 {
		token, err := h.generateCSRFToken(w, r)
		if err != nil {
			h.renderError(w, r, http.StatusInternalServerError, "Unable to generate security token")
			return
		}

		data := &PageData{
			Title:  "Share a link",
			User:   user,
			Errors: errorsList,
			FormValues: map[string]string{
				"title":   title,
				"url":     url,
				"content": content,
			},
			CSRFToken: token,
		}
		h.render(w, r, "new_post.gohtml", data)
		return
	}

	post := &database.Post{
		Title:      title,
		URL:        url,
		Content:    content,
		AuthorID:   &user.ID,
		AuthorName: user.Username,
	}

	if err := h.postRepo.Create(post); err != nil {
		data := &PageData{
			Title:  "Share a link",
			User:   user,
			Errors: []string{"Could not create the post. Please try again."},
			FormValues: map[string]string{
				"title":   title,
				"url":     url,
				"content": content,
			},
		}
		h.render(w, r, "new_post.gohtml", data)
		return
	}

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (h *PageHandler) LoginForm(w http.ResponseWriter, r *http.Request) {
	if h.currentUser(r) != nil {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	flash := strings.TrimSpace(r.URL.Query().Get("flash"))
	if flash == "" && r.URL.Query().Get("verified") != "" {
		flash = "Account verified. You can now sign in."
	}
	if flash == "" && r.URL.Query().Get("reset") == "success" {
		flash = "Password reset successfully. You can now sign in with your new password."
	}

	data := &PageData{
		Title:      "Sign in",
		FormValues: map[string]string{},
		Flash:      flash,
	}

	if err := h.setCSRFToken(w, r, data); err != nil {
		h.renderError(w, r, http.StatusInternalServerError, "Unable to generate security token")
		return
	}

	h.render(w, r, "login.gohtml", data)
}

func (h *PageHandler) Login(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		h.renderError(w, r, http.StatusBadRequest, "Invalid form submission")
		return
	}

	username := strings.TrimSpace(r.FormValue("username"))
	password := r.FormValue("password")

	var errorsList []string
	if username == "" {
		errorsList = append(errorsList, "Username is required")
	}
	if strings.TrimSpace(password) == "" {
		errorsList = append(errorsList, "Password is required")
	}

	if len(errorsList) > 0 {
		data := &PageData{
			Title:  "Sign in",
			Errors: errorsList,
			FormValues: map[string]string{
				"username": username,
			},
		}
		if err := h.setCSRFToken(w, r, data); err != nil {
			h.renderError(w, r, http.StatusInternalServerError, "Unable to generate security token")
			return
		}
		h.render(w, r, "login.gohtml", data)
		return
	}

	if h.authService == nil {
		h.renderError(w, r, http.StatusInternalServerError, "Authentication service is not available")
		return
	}

	result, err := h.authService.Login(username, password)
	if err != nil {
		message := strings.TrimSpace(err.Error())
		if errors.Is(err, services.ErrInvalidCredentials) {
			message = "Invalid username or password"
		} else if errors.Is(err, services.ErrEmailNotVerified) {
			message = "Please confirm your email before signing in"
		} else if errors.Is(err, services.ErrAccountLocked) {
			message = "Your account has been locked. Please contact us for assistance."
		}
		if message == "" {
			message = "Unable to sign you in right now. Please try again."
		}
		data := &PageData{
			Title:  "Sign in",
			Errors: []string{message},
			FormValues: map[string]string{
				"username": username,
			},
		}
		if err := h.setCSRFToken(w, r, data); err != nil {
			h.renderError(w, r, http.StatusInternalServerError, "Unable to generate security token")
			return
		}
		h.render(w, r, "login.gohtml", data)
		return
	}

	cookie := &http.Cookie{
		Name:     "auth_token",
		Value:    result.AccessToken,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   middleware.IsHTTPS(r),
		Expires:  time.Now().Add(24 * time.Hour),
	}
	http.SetCookie(w, cookie)

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (h *PageHandler) RegisterForm(w http.ResponseWriter, r *http.Request) {
	if h.currentUser(r) != nil {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	data := &PageData{
		Title:      "Create account",
		FormValues: map[string]string{"email": ""},
	}

	if err := h.setCSRFToken(w, r, data); err != nil {
		h.renderError(w, r, http.StatusInternalServerError, "Unable to generate security token")
		return
	}

	h.render(w, r, "register.gohtml", data)
}

func (h *PageHandler) Register(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		h.renderError(w, r, http.StatusBadRequest, "Invalid form submission")
		return
	}

	username := strings.TrimSpace(r.FormValue("username"))
	email := strings.TrimSpace(r.FormValue("email"))
	password := r.FormValue("password")
	confirm := r.FormValue("password_confirm")

	var errorsList []string
	if username == "" {
		errorsList = append(errorsList, "Username is required")
	}
	if email == "" {
		errorsList = append(errorsList, "Email is required")
	}
	if strings.TrimSpace(password) == "" {
		errorsList = append(errorsList, "Password is required")
	}
	if password != confirm {
		errorsList = append(errorsList, "Passwords do not match")
	}

	if len(errorsList) > 0 {
		data := &PageData{
			Title:  "Create account",
			Errors: errorsList,
			FormValues: map[string]string{
				"username": username,
				"email":    email,
			},
		}
		if err := h.setCSRFToken(w, r, data); err != nil {
			h.renderError(w, r, http.StatusInternalServerError, "Unable to generate security token")
			return
		}
		h.render(w, r, "register.gohtml", data)
		return
	}

	if h.authService == nil {
		h.renderError(w, r, http.StatusInternalServerError, "Authentication service is not available")
		return
	}

	_, err := h.authService.Register(username, email, password)
	if err != nil {
		message := strings.TrimSpace(err.Error())
		switch {
		case errors.Is(err, services.ErrUsernameTaken):
			message = "That username is already taken. Try another one."
		case errors.Is(err, services.ErrEmailTaken):
			message = "That email is already registered. Try signing in or use another email."
		case message == "":
			message = "Unable to create the account right now. Please try again."
		}

		data := &PageData{
			Title:  "Create account",
			Errors: []string{message},
			FormValues: map[string]string{
				"username": username,
				"email":    email,
			},
		}
		if err := h.setCSRFToken(w, r, data); err != nil {
			h.renderError(w, r, http.StatusInternalServerError, "Unable to generate security token")
			return
		}
		h.render(w, r, "register.gohtml", data)
		return
	}

	data := &PageData{
		Title: "Sign in",
		Flash: "Account created. Check your inbox to confirm your email before signing in.",
		FormValues: map[string]string{
			"username": username,
		},
	}
	h.render(w, r, "login.gohtml", data)
}

func (h *PageHandler) ConfirmEmailPage(w http.ResponseWriter, r *http.Request) {
	token := strings.TrimSpace(r.URL.Query().Get("token"))

	data := &PageData{
		Title: "Confirm email",
	}

	if token == "" {
		data.Errors = []string{"The verification link is missing or invalid."}
		h.render(w, r, "confirm_email.gohtml", data)
		return
	}

	if h.authService == nil {
		h.renderError(w, r, http.StatusInternalServerError, "Email verification is not available right now")
		return
	}

	if _, err := h.authService.ConfirmEmail(token); err != nil {
		message := "We couldn't verify your account. The link may be invalid or expired."
		if !errors.Is(err, services.ErrInvalidVerificationToken) {
			message = "We couldn't verify your account right now. Please try again later."
		}
		data.Errors = []string{message}
	} else {
		data.VerificationSuccess = true
		data.Flash = "Account verified. You can now sign in."
	}

	h.render(w, r, "confirm_email.gohtml", data)
}

func (h *PageHandler) ResendVerificationForm(w http.ResponseWriter, r *http.Request) {
	data := &PageData{
		Title: "Resend verification email",
	}

	if err := h.setCSRFToken(w, r, data); err != nil {
		h.renderError(w, r, http.StatusInternalServerError, "Unable to generate security token")
		return
	}

	h.render(w, r, "resend_verification.gohtml", data)
}

func (h *PageHandler) ResendVerification(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		h.renderError(w, r, http.StatusBadRequest, "Invalid form data")
		return
	}

	email := strings.TrimSpace(r.FormValue("email"))

	data := &PageData{
		Title: "Resend verification email",
		FormValues: map[string]string{
			"email": email,
		},
	}

	if !middleware.ValidateCSRFToken(r) {
		data.Errors = []string{"Invalid security token. Please try again."}
		if err := h.setCSRFToken(w, r, data); err != nil {
			h.renderError(w, r, http.StatusInternalServerError, "Unable to generate security token")
			return
		}
		h.render(w, r, "resend_verification.gohtml", data)
		return
	}

	if email == "" {
		data.Errors = []string{"Email address is required."}
		if err := h.setCSRFToken(w, r, data); err != nil {
			h.renderError(w, r, http.StatusInternalServerError, "Unable to generate security token")
			return
		}
		h.render(w, r, "resend_verification.gohtml", data)
		return
	}

	if h.authService == nil {
		h.renderError(w, r, http.StatusInternalServerError, "Email verification is not available right now")
		return
	}

	err := h.authService.ResendVerificationEmail(email)
	if err != nil {
		message := "Unable to resend verification email. Please try again later."
		switch {
		case errors.Is(err, services.ErrInvalidCredentials):
			message = "No account found with this email address."
		case errors.Is(err, services.ErrInvalidEmail):
			message = "Please enter a valid email address."
		case err.Error() == "email already verified":
			message = "This email address is already verified. You can sign in now."
		case err.Error() == "verification email sent recently, please wait before requesting another":
			message = "Please wait 5 minutes before requesting another verification email."
		}
		data.Errors = []string{message}
		if err := h.setCSRFToken(w, r, data); err != nil {
			h.renderError(w, r, http.StatusInternalServerError, "Unable to generate security token")
			return
		}
		h.render(w, r, "resend_verification.gohtml", data)
		return
	}

	data.Flash = "Verification email sent! Check your inbox for the confirmation link."
	if err := h.setCSRFToken(w, r, data); err != nil {
		h.renderError(w, r, http.StatusInternalServerError, "Unable to generate security token")
		return
	}
	h.render(w, r, "resend_verification.gohtml", data)
}

func (h *PageHandler) ForgotPasswordForm(w http.ResponseWriter, r *http.Request) {
	data := &PageData{
		Title: "Reset password",
	}

	if err := h.setCSRFToken(w, r, data); err != nil {
		h.renderError(w, r, http.StatusInternalServerError, "Unable to generate security token")
		return
	}
	h.render(w, r, "forgot_password.gohtml", data)
}

func (h *PageHandler) ForgotPassword(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		h.renderError(w, r, http.StatusBadRequest, "Invalid form data")
		return
	}

	usernameOrEmail := strings.TrimSpace(r.FormValue("username_or_email"))

	data := &PageData{
		Title: "Reset password",
		FormValues: map[string]string{
			"username_or_email": usernameOrEmail,
		},
	}

	if err := h.setCSRFToken(w, r, data); err != nil {
		h.renderError(w, r, http.StatusInternalServerError, "Unable to generate security token")
		return
	}

	if usernameOrEmail == "" {
		data.Errors = []string{"Username or email address is required."}
		h.render(w, r, "forgot_password.gohtml", data)
		return
	}

	if h.authService == nil {
		h.renderError(w, r, http.StatusInternalServerError, "Password reset is not available right now")
		return
	}

	if err := h.authService.RequestPasswordReset(usernameOrEmail); err != nil {
		message := "Unable to send password reset email. Please try again later."
		if !strings.Contains(err.Error(), "email") {
			message = "Unable to send password reset email. Please try again later."
		}
		data.Errors = []string{message}
		h.render(w, r, "forgot_password.gohtml", data)
		return
	}

	data.Flash = "If an account with that username or email exists, we've sent a password reset link."
	h.render(w, r, "forgot_password.gohtml", data)
}

func (h *PageHandler) ResetPasswordForm(w http.ResponseWriter, r *http.Request) {
	token := strings.TrimSpace(r.URL.Query().Get("token"))

	data := &PageData{
		Title: "Set new password",
		Token: token,
	}

	if err := h.setCSRFToken(w, r, data); err != nil {
		h.renderError(w, r, http.StatusInternalServerError, "Unable to generate security token")
		return
	}

	if token == "" {
		data.Errors = []string{"The reset link is missing or invalid."}
		h.render(w, r, "reset_password.gohtml", data)
		return
	}

	h.render(w, r, "reset_password.gohtml", data)
}

func (h *PageHandler) ResetPassword(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		h.renderError(w, r, http.StatusBadRequest, "Invalid form data")
		return
	}

	token := strings.TrimSpace(r.FormValue("token"))
	password := strings.TrimSpace(r.FormValue("password"))
	confirmPassword := strings.TrimSpace(r.FormValue("confirm_password"))

	data := &PageData{
		Title: "Set new password",
		Token: token,
		FormValues: map[string]string{
			"password": password,
		},
	}

	if err := h.setCSRFToken(w, r, data); err != nil {
		h.renderError(w, r, http.StatusInternalServerError, "Unable to generate security token")
		return
	}

	if token == "" {
		data.Errors = []string{"The reset link is missing or invalid."}
		h.render(w, r, "reset_password.gohtml", data)
		return
	}

	if password == "" {
		data.Errors = []string{"Password is required."}
		h.render(w, r, "reset_password.gohtml", data)
		return
	}

	if err := validation.ValidatePassword(password); err != nil {
		data.Errors = []string{err.Error()}
		h.render(w, r, "reset_password.gohtml", data)
		return
	}

	if password != confirmPassword {
		data.Errors = []string{"Passwords do not match."}
		h.render(w, r, "reset_password.gohtml", data)
		return
	}

	if h.authService == nil {
		h.renderError(w, r, http.StatusInternalServerError, "Password reset is not available right now")
		return
	}

	if err := h.authService.ResetPassword(token, password); err != nil {
		message := "Unable to reset password. The link may be invalid or expired."
		if strings.Contains(err.Error(), "expired") {
			message = "The reset link has expired. Please request a new one."
		} else if strings.Contains(err.Error(), "invalid") {
			message = "The reset link is invalid. Please request a new one."
		}
		data.Errors = []string{message}
		h.render(w, r, "reset_password.gohtml", data)
		return
	}

	http.Redirect(w, r, "/login?reset=success", http.StatusSeeOther)
}

func (h *PageHandler) Settings(w http.ResponseWriter, r *http.Request) {
	user := h.currentUserWithLockCheck(w, r)
	if user == nil {
		redirectURL := "/login?flash=" + url.QueryEscape("Sign in to manage your account")
		http.Redirect(w, r, redirectURL, http.StatusSeeOther)
		return
	}

	data := h.settingsPageData(user)
	data.Flash = strings.TrimSpace(r.URL.Query().Get("flash"))

	token, err := h.generateCSRFToken(w, r)
	if err != nil {
		h.renderError(w, r, http.StatusInternalServerError, "Unable to generate security token")
		return
	}
	data.CSRFToken = token

	h.render(w, r, "settings.gohtml", data)
}

func (h *PageHandler) UpdateEmail(w http.ResponseWriter, r *http.Request) {
	user := h.currentUserWithLockCheck(w, r)
	if user == nil {
		redirectURL := "/login?flash=" + url.QueryEscape("Sign in to manage your account")
		http.Redirect(w, r, redirectURL, http.StatusSeeOther)
		return
	}

	if err := r.ParseForm(); err != nil {
		h.renderError(w, r, http.StatusBadRequest, "Invalid form submission")
		return
	}

	newEmail := strings.TrimSpace(r.FormValue("email"))
	data := h.settingsPageData(user)
	if data.FormErrors == nil {
		data.FormErrors = map[string][]string{}
	}
	data.FormValues["email"] = newEmail

	if newEmail == "" {
		token, err := h.generateCSRFToken(w, r)
		if err != nil {
			h.renderError(w, r, http.StatusInternalServerError, "Unable to generate security token")
			return
		}
		data.CSRFToken = token

		message := "Email is required"
		data.Errors = []string{message}
		data.setFormError("email", message)
		h.render(w, r, "settings.gohtml", data)
		return
	}

	if h.authService == nil {
		h.renderError(w, r, http.StatusInternalServerError, "Authentication service is not available")
		return
	}

	if _, err := h.authService.UpdateEmail(user.ID, newEmail); err != nil {
		token, tokenErr := h.generateCSRFToken(w, r)
		if tokenErr != nil {
			h.renderError(w, r, http.StatusInternalServerError, "Unable to generate security token")
			return
		}
		data.CSRFToken = token

		message := strings.TrimSpace(err.Error())
		switch {
		case errors.Is(err, services.ErrEmailTaken):
			message = "That email is already in use. Choose another one."
		case errors.Is(err, services.ErrEmailSenderUnavailable):
			message = "We couldn't send the confirmation email. Try again later."
		case message == "":
			message = "We couldn't update your email right now."
		}
		data.Errors = []string{message}
		data.setFormError("email", message)
		h.render(w, r, "settings.gohtml", data)
		return
	}

	if err := h.authService.InvalidateAllSessions(user.ID); err != nil {
	}
	h.clearAuthCookie(w, r)

	redirectURL := "/login?flash=" + url.QueryEscape("Email updated. Check your inbox to confirm the new address. You will need to sign in again after verification.")
	http.Redirect(w, r, redirectURL, http.StatusSeeOther)
}

func (h *PageHandler) UpdateUsername(w http.ResponseWriter, r *http.Request) {
	user := h.currentUserWithLockCheck(w, r)
	if user == nil {
		redirectURL := "/login?flash=" + url.QueryEscape("Sign in to manage your account")
		http.Redirect(w, r, redirectURL, http.StatusSeeOther)
		return
	}

	if err := r.ParseForm(); err != nil {
		h.renderError(w, r, http.StatusBadRequest, "Invalid form submission")
		return
	}

	newUsername := strings.TrimSpace(r.FormValue("username"))
	data := h.settingsPageData(user)
	if data.FormErrors == nil {
		data.FormErrors = map[string][]string{}
	}
	data.FormValues["username"] = newUsername

	if newUsername == "" {
		token, err := h.generateCSRFToken(w, r)
		if err != nil {
			h.renderError(w, r, http.StatusInternalServerError, "Unable to generate security token")
			return
		}
		data.CSRFToken = token

		message := "Username is required"
		data.Errors = []string{message}
		data.setFormError("username", message)
		h.render(w, r, "settings.gohtml", data)
		return
	}

	if h.authService == nil {
		h.renderError(w, r, http.StatusInternalServerError, "Authentication service is not available")
		return
	}

	if _, err := h.authService.UpdateUsername(user.ID, newUsername); err != nil {
		token, tokenErr := h.generateCSRFToken(w, r)
		if tokenErr != nil {
			h.renderError(w, r, http.StatusInternalServerError, "Unable to generate security token")
			return
		}
		data.CSRFToken = token

		message := strings.TrimSpace(err.Error())
		switch {
		case errors.Is(err, services.ErrUsernameTaken):
			message = "That username is already taken. Try another one."
		case message == "":
			message = "We couldn't update your username right now."
		}
		data.Errors = []string{message}
		h.render(w, r, "settings.gohtml", data)
		return
	}

	redirectURL := "/settings?flash=" + url.QueryEscape("Username updated successfully.")
	http.Redirect(w, r, redirectURL, http.StatusSeeOther)
}

func (h *PageHandler) UpdatePassword(w http.ResponseWriter, r *http.Request) {
	user := h.currentUserWithLockCheck(w, r)
	if user == nil {
		redirectURL := "/login?flash=" + url.QueryEscape("Sign in to manage your account")
		http.Redirect(w, r, redirectURL, http.StatusSeeOther)
		return
	}

	if err := r.ParseForm(); err != nil {
		h.renderError(w, r, http.StatusBadRequest, "Invalid form submission")
		return
	}

	currentPassword := strings.TrimSpace(r.FormValue("current_password"))
	newPassword := strings.TrimSpace(r.FormValue("new_password"))
	confirmPassword := strings.TrimSpace(r.FormValue("confirm_password"))

	data := h.settingsPageData(user)
	if data.FormErrors == nil {
		data.FormErrors = map[string][]string{}
	}

	data.FormValues["current_password"] = ""
	data.FormValues["new_password"] = ""
	data.FormValues["confirm_password"] = ""

	if currentPassword == "" {
		token, err := h.generateCSRFToken(w, r)
		if err != nil {
			h.renderError(w, r, http.StatusInternalServerError, "Unable to generate security token")
			return
		}
		data.CSRFToken = token

		message := "Current password is required"
		data.Errors = []string{message}
		data.setFormError("current_password", message)
		h.render(w, r, "settings.gohtml", data)
		return
	}

	if newPassword == "" {
		token, err := h.generateCSRFToken(w, r)
		if err != nil {
			h.renderError(w, r, http.StatusInternalServerError, "Unable to generate security token")
			return
		}
		data.CSRFToken = token

		message := "New password is required"
		data.Errors = []string{message}
		data.setFormError("new_password", message)
		h.render(w, r, "settings.gohtml", data)
		return
	}

	if len(newPassword) < 8 {
		token, err := h.generateCSRFToken(w, r)
		if err != nil {
			h.renderError(w, r, http.StatusInternalServerError, "Unable to generate security token")
			return
		}
		data.CSRFToken = token

		message := "New password must be at least 8 characters long"
		data.Errors = []string{message}
		data.setFormError("new_password", message)
		h.render(w, r, "settings.gohtml", data)
		return
	}

	if newPassword != confirmPassword {
		token, err := h.generateCSRFToken(w, r)
		if err != nil {
			h.renderError(w, r, http.StatusInternalServerError, "Unable to generate security token")
			return
		}
		data.CSRFToken = token

		message := "New passwords do not match"
		data.Errors = []string{message}
		data.setFormError("confirm_password", message)
		h.render(w, r, "settings.gohtml", data)
		return
	}

	if h.authService == nil {
		h.renderError(w, r, http.StatusInternalServerError, "Authentication service is not available")
		return
	}

	if _, err := h.authService.UpdatePassword(user.ID, currentPassword, newPassword); err != nil {
		token, tokenErr := h.generateCSRFToken(w, r)
		if tokenErr != nil {
			h.renderError(w, r, http.StatusInternalServerError, "Unable to generate security token")
			return
		}
		data.CSRFToken = token

		message := strings.TrimSpace(err.Error())
		switch {
		case strings.Contains(message, "current password is incorrect"):
			message = "Current password is incorrect"
		case message == "":
			message = "We couldn't update your password right now."
		}
		data.Errors = []string{message}
		if strings.Contains(err.Error(), "current password") {
			data.setFormError("current_password", message)
		} else {
			data.setFormError("new_password", message)
		}
		h.render(w, r, "settings.gohtml", data)
		return
	}

	redirectURL := "/settings?flash=" + url.QueryEscape("Password updated successfully.")
	http.Redirect(w, r, redirectURL, http.StatusSeeOther)
}

func (h *PageHandler) DeleteAccount(w http.ResponseWriter, r *http.Request) {
	user := h.currentUserWithLockCheck(w, r)
	if user == nil {
		redirectURL := "/login?flash=" + url.QueryEscape("Sign in to manage your account")
		http.Redirect(w, r, redirectURL, http.StatusSeeOther)
		return
	}

	if err := r.ParseForm(); err != nil {
		h.renderError(w, r, http.StatusBadRequest, "Invalid form submission")
		return
	}

	confirmation := strings.TrimSpace(r.FormValue("confirmation"))
	data := h.settingsPageData(user)
	if data.FormErrors == nil {
		data.FormErrors = map[string][]string{}
	}

	if !strings.EqualFold(confirmation, "DELETE") {
		token, err := h.generateCSRFToken(w, r)
		if err != nil {
			h.renderError(w, r, http.StatusInternalServerError, "Unable to generate security token")
			return
		}
		data.CSRFToken = token

		message := "Type DELETE in capital letters to confirm."
		data.Errors = []string{message}
		data.setFormError("delete", message)
		h.render(w, r, "settings.gohtml", data)
		return
	}

	if h.authService == nil {
		h.renderError(w, r, http.StatusInternalServerError, "Authentication service is not available")
		return
	}

	if err := h.authService.RequestAccountDeletion(user.ID); err != nil {
		token, tokenErr := h.generateCSRFToken(w, r)
		if tokenErr != nil {
			h.renderError(w, r, http.StatusInternalServerError, "Unable to generate security token")
			return
		}
		data.CSRFToken = token

		message := strings.TrimSpace(err.Error())
		switch {
		case errors.Is(err, services.ErrEmailSenderUnavailable):
			message = "Account deletion isn't available right now because email delivery is disabled."
		case errors.Is(err, services.ErrInvalidDeletionToken):
			message = "We couldn't start the deletion process. Please try again."
		case message == "":
			message = "We couldn't start the deletion process right now."
		}
		data.Errors = []string{message}
		data.setFormError("delete", message)
		h.render(w, r, "settings.gohtml", data)
		return
	}

	redirectURL := "/settings?flash=" + url.QueryEscape("Check your inbox for a confirmation link to finish deleting your account.")
	http.Redirect(w, r, redirectURL, http.StatusSeeOther)
}

func (h *PageHandler) ConfirmAccountDeletion(w http.ResponseWriter, r *http.Request) {
	token := strings.TrimSpace(r.URL.Query().Get("token"))

	if token == "" && r.Method == http.MethodPost {
		if err := r.ParseForm(); err == nil {
			token = strings.TrimSpace(r.FormValue("token"))
		}
	}

	data := &PageData{
		Title: "Confirm account deletion",
	}

	if token == "" {
		data.Errors = []string{"The deletion link is missing or invalid."}
		h.render(w, r, "confirm_delete.gohtml", data)
		return
	}

	if h.authService == nil {
		h.renderError(w, r, http.StatusInternalServerError, "Account deletion is not available right now")
		return
	}

	if r.Method == http.MethodPost {
		deletePostsStr := r.FormValue("delete_posts")
		deletePosts := deletePostsStr == "true"

		if err := h.authService.ConfirmAccountDeletionWithPosts(token, deletePosts); err != nil {
			switch {
			case errors.Is(err, services.ErrInvalidDeletionToken):
				data.Errors = []string{"This deletion link is invalid or has expired."}
			case errors.Is(err, services.ErrEmailSenderUnavailable):
				data.Errors = []string{"Account deletion is currently unavailable because email delivery is disabled."}
			case errors.Is(err, services.ErrDeletionEmailFailed):
				h.clearAuthCookie(w, r)
				data.Flash = "Your account has been deleted, but we couldn't send the confirmation email."
				data.ShowLoginLinks = true
			default:
				data.Errors = []string{"We couldn't confirm the deletion right now. Please try again later."}
			}
			h.render(w, r, "confirm_delete.gohtml", data)
			return
		}

		h.clearAuthCookie(w, r)
		data.Flash = "Your account has been deleted."
		data.ShowLoginLinks = true
		h.render(w, r, "confirm_delete.gohtml", data)
		return
	}

	hasPosts, postCount, err := h.validateDeletionTokenAndCheckPosts(token)
	if err != nil {
		switch {
		case errors.Is(err, services.ErrInvalidDeletionToken):
			data.Errors = []string{"This deletion link is invalid or has expired."}
		default:
			data.Errors = []string{"We couldn't validate the deletion link. Please try again later."}
		}
		h.render(w, r, "confirm_delete.gohtml", data)
		return
	}

	data.Token = token
	data.HasPosts = hasPosts
	data.PostCount = postCount
	h.render(w, r, "confirm_delete.gohtml", data)
}

func (h *PageHandler) validateDeletionTokenAndCheckPosts(token string) (bool, int64, error) {
	if h.authService == nil {
		return false, 0, fmt.Errorf("auth service not available")
	}

	userID, err := h.authService.GetUserIDFromDeletionToken(token)
	if err != nil {
		return false, 0, err
	}

	return h.authService.UserHasPosts(userID)
}

func (h *PageHandler) Logout(w http.ResponseWriter, r *http.Request) {
	h.clearAuthCookie(w, r)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (h *PageHandler) settingsPageData(user *database.User) *PageData {
	formValues := map[string]string{}
	if user != nil {
		formValues["email"] = user.Email
		formValues["username"] = user.Username
	}

	return &PageData{
		Title:      "Account settings",
		SiteTitle:  h.config.App.Title,
		User:       user,
		FormValues: formValues,
		FormErrors: map[string][]string{},
	}
}

func (h *PageHandler) clearAuthCookie(w http.ResponseWriter, r *http.Request) {
	cookie := &http.Cookie{
		Name:     "auth_token",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   middleware.IsHTTPS(r),
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
		SameSite: http.SameSiteLaxMode,
	}
	http.SetCookie(w, cookie)
}

func (h *PageHandler) Vote(w http.ResponseWriter, r *http.Request) {
	user := h.currentUserWithLockCheck(w, r)
	if user == nil {
		redirectURL := "/login?flash=" + url.QueryEscape("Please sign in to vote")
		http.Redirect(w, r, redirectURL, http.StatusSeeOther)
		return
	}

	if err := r.ParseForm(); err != nil {
		h.renderError(w, r, http.StatusBadRequest, "Invalid vote submission")
		return
	}

	postIDStr := chi.URLParam(r, "id")
	postID, err := strconv.Atoi(postIDStr)
	if err != nil || postID <= 0 {
		h.renderError(w, r, http.StatusBadRequest, "Invalid post identifier")
		return
	}

	action := strings.TrimSpace(r.FormValue("action"))

	ipAddress := GetClientIP(r)
	userAgent := r.UserAgent()

	userID := user.ID

	var voteType database.VoteType
	switch action {
	case "up":
		voteType = database.VoteUp
	case "down":
		voteType = database.VoteDown
	case "clear":
		voteType = database.VoteNone
	default:
		h.renderError(w, r, http.StatusBadRequest, "Unsupported vote action")
		return
	}

	serviceReq := services.VoteRequest{
		UserID:    userID,
		PostID:    uint(postID),
		Type:      voteType,
		IPAddress: ipAddress,
		UserAgent: userAgent,
	}

	_, err = h.voteService.CastVote(serviceReq)
	if err != nil {
		h.renderError(w, r, http.StatusInternalServerError, "Unable to update the vote")
		return
	}

	redirectTarget := ValidateRedirectURL(r.FormValue("redirect"))
	if redirectTarget == "" {
		redirectTarget = "/posts/" + strconv.Itoa(postID)
	}

	http.Redirect(w, r, redirectTarget, http.StatusSeeOther)
}

func (h *PageHandler) currentUser(r *http.Request) *database.User {
	cookie, err := r.Cookie("auth_token")
	if err != nil || strings.TrimSpace(cookie.Value) == "" {
		return nil
	}

	if h.authService == nil {
		return nil
	}

	userID, err := h.authService.VerifyToken(cookie.Value)
	if err != nil || userID == 0 {
		return nil
	}

	user, err := h.userRepo.GetByID(userID)
	if err != nil {
		return nil
	}

	user.Password = ""
	return user
}

func (h *PageHandler) currentUserWithLockCheck(w http.ResponseWriter, r *http.Request) *database.User {
	cookie, err := r.Cookie("auth_token")
	if err != nil || strings.TrimSpace(cookie.Value) == "" {
		return nil
	}

	if h.authService == nil {
		return nil
	}

	userID, err := h.authService.VerifyToken(cookie.Value)
	if err != nil || userID == 0 {
		if errors.Is(err, services.ErrAccountLocked) {
			h.clearAuthCookie(w, r)
		}
		return nil
	}

	user, err := h.userRepo.GetByID(userID)
	if err != nil {
		return nil
	}

	user.Password = ""
	return user
}

func (h *PageHandler) generateCSRFToken(w http.ResponseWriter, r *http.Request) (string, error) {
	token, err := middleware.CSRFToken()
	if err != nil {
		return "", err
	}

	middleware.SetCSRFToken(w, r, token)

	return token, nil
}

func (h *PageHandler) setCSRFToken(w http.ResponseWriter, r *http.Request, data *PageData) error {
	token, err := h.generateCSRFToken(w, r)
	if err != nil {
		return err
	}

	if data != nil {
		data.CSRFToken = token
	}

	return nil
}

func (h *PageHandler) render(w http.ResponseWriter, r *http.Request, templateName string, data *PageData) {
	tmpl, err := h.getTemplate(templateName)
	if err != nil {
		h.renderError(w, r, http.StatusInternalServerError, "Template rendering error")
		return
	}

	if data == nil {
		data = &PageData{}
	}

	if data.FormValues == nil {
		data.FormValues = map[string]string{}
	}

	if data.FormErrors == nil {
		data.FormErrors = map[string][]string{}
	}

	data.CSPNonce = middleware.GetCSPNonceFromContext(r.Context())

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.ExecuteTemplate(w, "layout", data); err != nil {
		h.renderError(w, r, http.StatusInternalServerError, "Template rendering error")
		return
	}
}

func (h *PageHandler) renderError(w http.ResponseWriter, r *http.Request, status int, message string) {
	w.WriteHeader(status)
	tmpl, err := h.getTemplate("error.gohtml")
	if err != nil {
		http.Error(w, message, status)
		return
	}

	data := &PageData{
		Title:    http.StatusText(status),
		Errors:   []string{message},
		CSPNonce: middleware.GetCSPNonceFromContext(r.Context()),
	}

	if err := tmpl.ExecuteTemplate(w, "layout", data); err != nil {
		http.Error(w, message, status)
	}
}

func (h *PageHandler) reloadTemplates() error {
	h.mu.Lock()
	defer h.mu.Unlock()

	layoutPath := filepath.Join(h.templatesDir, "base.gohtml")
	if _, err := os.Stat(layoutPath); err != nil {
		return err
	}

	partials, err := filepath.Glob(filepath.Join(h.templatesDir, "partials", "*.gohtml"))
	if err != nil {
		return err
	}

	pages, err := filepath.Glob(filepath.Join(h.templatesDir, "*.gohtml"))
	if err != nil {
		return err
	}

	templates := make(map[string]*template.Template)
	for _, page := range pages {
		if filepath.Base(page) == "base.gohtml" {
			continue
		}

		files := append([]string{layoutPath}, partials...)
		files = append(files, page)

		tmpl, parseErr := template.New(filepath.Base(page)).Funcs(h.funcMap).ParseFiles(files...)
		if parseErr != nil {
			return parseErr
		}

		templates[filepath.Base(page)] = tmpl
	}

	if len(templates) == 0 {
		return errors.New("no templates were loaded")
	}

	h.templates = templates
	return nil
}

func (h *PageHandler) getTemplate(name string) (*template.Template, error) {
	h.mu.RLock()
	tmpl, ok := h.templates[name]
	h.mu.RUnlock()
	if ok {
		return tmpl, nil
	}

	if err := h.reloadTemplates(); err != nil {
		return nil, err
	}

	h.mu.RLock()
	defer h.mu.RUnlock()
	tmpl, ok = h.templates[name]
	if !ok {
		return nil, errors.New("template not found: " + name)
	}

	return tmpl, nil
}

func HSTSMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if middleware.IsHTTPS(r) {
			w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}
		next.ServeHTTP(w, r)
	})
}

func (h *PageHandler) MountRoutes(r chi.Router, config RouteModuleConfig) {
	public := r
	if config.GeneralRateLimit != nil {
		public = config.GeneralRateLimit(r)
	}

	public.Get("/", h.Home)
	public.Get("/search", h.Search)
	public.Get("/login", h.LoginForm)
	public.Get("/register", h.RegisterForm)
	public.Get("/confirm", h.ConfirmEmailPage)
	public.Get("/resend-verification", h.ResendVerificationForm)
	public.Get("/forgot-password", h.ForgotPasswordForm)
	public.Get("/reset-password", h.ResetPasswordForm)
	public.Get("/settings/delete/confirm", h.ConfirmAccountDeletion)
	public.Get("/posts/new", h.NewPostForm)
	public.Get("/posts/{id:[0-9]+}", h.ShowPost)

	protected := r
	if config.CSRFMiddleware != nil {
		protected = protected.With(config.CSRFMiddleware)
	}
	if config.AuthRateLimit != nil {
		protected = config.AuthRateLimit(protected)
	}

	protected.Post("/login", h.Login)
	protected.Post("/logout", h.Logout)
	protected.Post("/register", h.Register)
	protected.Post("/resend-verification", h.ResendVerification)
	protected.Post("/forgot-password", h.ForgotPassword)
	protected.Post("/reset-password", h.ResetPassword)
	protected.Get("/settings", h.Settings)
	protected.Post("/settings/email", h.UpdateEmail)
	protected.Post("/settings/username", h.UpdateUsername)
	protected.Post("/settings/password", h.UpdatePassword)
	protected.Post("/settings/delete", h.DeleteAccount)
	protected.Post("/settings/delete/confirm", h.ConfirmAccountDeletion)
	protected.Post("/posts", h.CreatePost)
	protected.Post("/posts/{id:[0-9]+}/vote", h.Vote)
}
