package main

import (
	"context"
	"crypto/tls"
	"errors"
	"flag"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"goyco/internal/config"
	"goyco/internal/database"
	"goyco/internal/handlers"
	"goyco/internal/repositories"
	"goyco/internal/server"
	"goyco/internal/services"
	"goyco/internal/testutils"
)

func TestServerConfigurationFromConfig(t *testing.T) {
	cfg := testutils.NewTestConfig()
	cfg.Server.ReadTimeout = 30 * time.Second
	cfg.Server.WriteTimeout = 30 * time.Second
	cfg.Server.IdleTimeout = 120 * time.Second
	cfg.Server.MaxHeaderBytes = 1 << 20

	db := testutils.NewTestDB(t)
	defer func() {
		sqlDB, _ := db.DB()
		_ = sqlDB.Close()
	}()

	userRepo := repositories.NewUserRepository(db)
	postRepo := repositories.NewPostRepository(db)
	voteRepo := repositories.NewVoteRepository(db)
	deletionRepo := repositories.NewAccountDeletionRepository(db)
	refreshTokenRepo := repositories.NewRefreshTokenRepository(db)
	emailSender := &testutils.MockEmailSender{}

	authService, err := services.NewAuthFacadeForTest(cfg, userRepo, postRepo, deletionRepo, refreshTokenRepo, emailSender)
	if err != nil {
		t.Fatalf("Failed to create auth service: %v", err)
	}

	voteService := services.NewVoteService(voteRepo, postRepo, db)
	metadataService := services.NewURLMetadataService()

	authHandler := handlers.NewAuthHandler(authService, userRepo)
	postHandler := handlers.NewPostHandler(postRepo, metadataService, voteService)
	voteHandler := handlers.NewVoteHandler(voteService)
	userHandler := handlers.NewUserHandler(userRepo, authService)
	apiHandler := handlers.NewAPIHandler(cfg, postRepo, userRepo, voteService)

	router := server.NewRouter(server.RouterConfig{
		AuthHandler:        authHandler,
		PostHandler:        postHandler,
		VoteHandler:        voteHandler,
		UserHandler:        userHandler,
		APIHandler:         apiHandler,
		AuthService:        authService,
		StaticDir:          "./internal/static/",
		Debug:              cfg.App.Debug,
		DisableCache:       true,
		DisableCompression: true,
		RateLimitConfig:    cfg.RateLimit,
	})

	srv := &http.Server{
		Addr:           cfg.Server.Host + ":" + cfg.Server.Port,
		Handler:        router,
		ReadTimeout:    cfg.Server.ReadTimeout,
		WriteTimeout:   cfg.Server.WriteTimeout,
		IdleTimeout:    cfg.Server.IdleTimeout,
		MaxHeaderBytes: cfg.Server.MaxHeaderBytes,
	}

	if srv.ReadTimeout != 30*time.Second {
		t.Errorf("Expected ReadTimeout to be 30s, got %v", srv.ReadTimeout)
	}

	if srv.WriteTimeout != 30*time.Second {
		t.Errorf("Expected WriteTimeout to be 30s, got %v", srv.WriteTimeout)
	}

	if srv.IdleTimeout != 120*time.Second {
		t.Errorf("Expected IdleTimeout to be 120s, got %v", srv.IdleTimeout)
	}

	if srv.MaxHeaderBytes != 1<<20 {
		t.Errorf("Expected MaxHeaderBytes to be 1MB, got %d", srv.MaxHeaderBytes)
	}

	testServer := httptest.NewServer(srv.Handler)
	defer testServer.Close()

	request, err := http.NewRequestWithContext(context.Background(), http.MethodGet, testServer.URL+"/health", nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatalf("Failed to make request: %v", err)
	}
	defer func() {
		_ = response.Body.Close()
	}()

	if response.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", response.StatusCode)
	}
}

func TestTLSWiringFromConfig(t *testing.T) {
	cfg := testutils.NewTestConfig()
	cfg.Server.EnableTLS = true
	cfg.Server.TLSCertFile = "/tmp/nonexistent-cert.pem"
	cfg.Server.TLSKeyFile = "/tmp/nonexistent-key.pem"

	db := testutils.NewTestDB(t)
	defer func() {
		sqlDB, _ := db.DB()
		_ = sqlDB.Close()
	}()

	userRepo := repositories.NewUserRepository(db)
	postRepo := repositories.NewPostRepository(db)
	voteRepo := repositories.NewVoteRepository(db)
	deletionRepo := repositories.NewAccountDeletionRepository(db)
	refreshTokenRepo := repositories.NewRefreshTokenRepository(db)
	emailSender := &testutils.MockEmailSender{}

	authService, err := services.NewAuthFacadeForTest(cfg, userRepo, postRepo, deletionRepo, refreshTokenRepo, emailSender)
	if err != nil {
		t.Fatalf("Failed to create auth service: %v", err)
	}

	voteService := services.NewVoteService(voteRepo, postRepo, db)
	metadataService := services.NewURLMetadataService()

	authHandler := handlers.NewAuthHandler(authService, userRepo)
	postHandler := handlers.NewPostHandler(postRepo, metadataService, voteService)
	voteHandler := handlers.NewVoteHandler(voteService)
	userHandler := handlers.NewUserHandler(userRepo, authService)
	apiHandler := handlers.NewAPIHandler(cfg, postRepo, userRepo, voteService)

	router := server.NewRouter(server.RouterConfig{
		AuthHandler:        authHandler,
		PostHandler:        postHandler,
		VoteHandler:        voteHandler,
		UserHandler:        userHandler,
		APIHandler:         apiHandler,
		AuthService:        authService,
		StaticDir:          "./internal/static/",
		Debug:              cfg.App.Debug,
		DisableCache:       true,
		DisableCompression: true,
		RateLimitConfig:    cfg.RateLimit,
	})

	expectedAddr := cfg.Server.Host + ":" + cfg.Server.Port
	srv := &http.Server{
		Addr:              expectedAddr,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
	}

	if srv.Addr != expectedAddr {
		t.Errorf("Expected server address to be %q, got %q", expectedAddr, srv.Addr)
	}

	if cfg.Server.EnableTLS {
		srv.TLSConfig = &tls.Config{
			MinVersion: tls.VersionTLS12,
			CipherSuites: []uint16{
				tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
				tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
				tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
				tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,
			},
		}

		if srv.TLSConfig == nil {
			t.Error("Expected TLS config to be set")
		}

		if srv.TLSConfig.MinVersion < tls.VersionTLS12 {
			t.Error("Expected minimum TLS version to be 1.2 or higher")
		}

		if len(srv.TLSConfig.CipherSuites) == 0 {
			t.Error("Expected cipher suites to be configured")
		}

		testServer := httptest.NewUnstartedServer(srv.Handler)
		testServer.TLS = srv.TLSConfig
		testServer.StartTLS()
		defer testServer.Close()

		client := &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{
					InsecureSkipVerify: true,
				},
			},
		}

		request, err := http.NewRequestWithContext(context.Background(), http.MethodGet, testServer.URL+"/health", nil)
		if err != nil {
			t.Fatalf("Failed to create request: %v", err)
		}

		response, err := client.Do(request)
		if err != nil {
			t.Fatalf("Failed to make TLS request: %v", err)
		}
		defer func() {
			_ = response.Body.Close()
		}()

		if response.StatusCode != http.StatusOK {
			t.Errorf("Expected status 200 over TLS, got %d", response.StatusCode)
		}

		if response.TLS == nil {
			t.Error("Expected TLS connection info to be present in response")
		} else if response.TLS.Version < tls.VersionTLS12 {
			t.Errorf("Expected TLS version 1.2 or higher, got %x", response.TLS.Version)
		}
	}
}

func TestConfigLoadingInCLI(t *testing.T) {
	originalEnv := os.Environ()
	defer func() {
		os.Clearenv()
		for _, env := range originalEnv {
			parts := splitEnv(env)
			if len(parts) == 2 {
				_ = os.Setenv(parts[0], parts[1])
			}
		}
	}()

	os.Clearenv()
	_ = os.Setenv("DB_PASSWORD", "test-password-123")
	_ = os.Setenv("SMTP_HOST", "smtp.example.com")
	_ = os.Setenv("SMTP_FROM", "test@example.com")
	_ = os.Setenv("ADMIN_EMAIL", "admin@example.com")
	_ = os.Setenv("JWT_SECRET", "test-jwt-secret-key-that-is-long-enough-for-validation")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	if cfg.Server.Port == "" {
		t.Error("Expected server port to be set")
	}

	if cfg.Database.Host == "" {
		t.Error("Expected database host to be set")
	}
}

func TestFlagParsingInCLI(t *testing.T) {
	originalArgs := os.Args
	defer func() {
		os.Args = originalArgs
	}()

	t.Run("help flag", func(t *testing.T) {
		os.Args = []string{"goyco", "--help"}
		fs := flag.NewFlagSet("goyco", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		showHelp := fs.Bool("help", false, "show help")

		err := fs.Parse([]string{"--help"})
		if err != nil && !errors.Is(err, flag.ErrHelp) {
			t.Errorf("Expected help flag parsing, got error: %v", err)
		}

		if !*showHelp {
			t.Error("Expected help flag to be true")
		}
	})

	t.Run("command dispatch", func(t *testing.T) {
		cfg := testutils.NewTestConfig()

		err := dispatchCommand(cfg, "unknown", []string{})
		if err == nil {
			t.Error("Expected error for unknown command")
		}

		err = dispatchCommand(cfg, "help", []string{})
		if err != nil {
			t.Errorf("Help command should not error: %v", err)
		}
	})
}

func TestServerInitializationFlow(t *testing.T) {
	cfg := testutils.NewTestConfig()
	cfg.Server.Port = "0"

	db := testutils.NewTestDB(t)
	defer func() {
		sqlDB, _ := db.DB()
		_ = sqlDB.Close()
	}()

	if err := database.Migrate(db); err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}

	userRepo := repositories.NewUserRepository(db)
	postRepo := repositories.NewPostRepository(db)
	voteRepo := repositories.NewVoteRepository(db)
	deletionRepo := repositories.NewAccountDeletionRepository(db)
	refreshTokenRepo := repositories.NewRefreshTokenRepository(db)
	emailSender := &testutils.MockEmailSender{}

	authService, err := services.NewAuthFacadeForTest(cfg, userRepo, postRepo, deletionRepo, refreshTokenRepo, emailSender)
	if err != nil {
		t.Fatalf("Failed to create auth service: %v", err)
	}

	voteService := services.NewVoteService(voteRepo, postRepo, db)
	metadataService := services.NewURLMetadataService()

	authHandler := handlers.NewAuthHandler(authService, userRepo)
	postHandler := handlers.NewPostHandler(postRepo, metadataService, voteService)
	voteHandler := handlers.NewVoteHandler(voteService)
	userHandler := handlers.NewUserHandler(userRepo, authService)
	apiHandler := handlers.NewAPIHandler(cfg, postRepo, userRepo, voteService)

	router := server.NewRouter(server.RouterConfig{
		AuthHandler:        authHandler,
		PostHandler:        postHandler,
		VoteHandler:        voteHandler,
		UserHandler:        userHandler,
		APIHandler:         apiHandler,
		AuthService:        authService,
		StaticDir:          "./internal/static/",
		Debug:              cfg.App.Debug,
		DisableCache:       true,
		DisableCompression: true,
		RateLimitConfig:    cfg.RateLimit,
	})

	srv := &http.Server{
		Addr:           cfg.Server.Host + ":" + cfg.Server.Port,
		Handler:        router,
		ReadTimeout:    cfg.Server.ReadTimeout,
		WriteTimeout:   cfg.Server.WriteTimeout,
		IdleTimeout:    cfg.Server.IdleTimeout,
		MaxHeaderBytes: cfg.Server.MaxHeaderBytes,
	}

	if srv.Handler == nil {
		t.Error("Expected server handler to be set")
	}

	testServer := httptest.NewServer(srv.Handler)
	defer testServer.Close()

	request, err := http.NewRequestWithContext(context.Background(), http.MethodGet, testServer.URL+"/health", nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatalf("Failed to make request: %v", err)
	}
	defer func() {
		_ = response.Body.Close()
	}()

	if response.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", response.StatusCode)
	}

	request, err = http.NewRequestWithContext(context.Background(), http.MethodGet, testServer.URL+"/api", nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	response, err = http.DefaultClient.Do(request)
	if err != nil {
		t.Fatalf("Failed to make request: %v", err)
	}
	defer func() {
		_ = response.Body.Close()
	}()

	if response.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200 for API endpoint, got %d", response.StatusCode)
	}
}

func splitEnv(env string) []string {
	for i := 0; i < len(env); i++ {
		if env[i] == '=' {
			return []string{env[:i], env[i+1:]}
		}
	}
	return []string{env}
}
