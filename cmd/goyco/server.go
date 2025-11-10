package main

import (
	"crypto/tls"
	"fmt"
	"log"
	"net/http"

	"goyco/cmd/goyco/commands"
	"goyco/internal/config"
	"goyco/internal/database"
	"goyco/internal/handlers"
	"goyco/internal/middleware"
	"goyco/internal/repositories"
	"goyco/internal/server"
	"goyco/internal/services"

	_ "goyco/docs"
)

func runServerImpl(cfg *config.Config, daemon bool) error {
	if daemon {
		if err := commands.SetupDaemonLogging(cfg, cfg.LogDir); err != nil {
			return fmt.Errorf("setup daemon logging: %w", err)
		}
	}

	dbMonitor := middleware.NewInMemoryDBMonitor()

	poolManager, err := database.ConnectWithPool(cfg)
	if err != nil {
		return fmt.Errorf("connect to database: %w", err)
	}
	defer func() {
		middleware.StopAllRateLimiters()
		if err := poolManager.Close(); err != nil {
			log.Printf("Error closing database pool: %v", err)
		}
	}()

	db := poolManager.GetDB()

	if err := database.Migrate(db); err != nil {
		return fmt.Errorf("run migrations: %w", err)
	}

	if monitor := dbMonitor; monitor != nil {
		monitoringPlugin := database.NewGormDBMonitor(monitor)
		if err := db.Use(monitoringPlugin); err != nil {
			return fmt.Errorf("failed to add monitoring plugin: %w", err)
		}
	}

	voteRepository := repositories.NewVoteRepository(db)
	postRepository := repositories.NewPostRepository(db)
	userRepository := repositories.NewUserRepository(db)
	deletionRepository := repositories.NewAccountDeletionRepository(db)
	refreshTokenRepository := repositories.NewRefreshTokenRepository(db)

	emailSender := services.NewSMTPSender(cfg.SMTP.Host, cfg.SMTP.Port, cfg.SMTP.Username, cfg.SMTP.Password, cfg.SMTP.From)
	emailService, err := services.NewEmailService(cfg, emailSender)
	if err != nil {
		return fmt.Errorf("create email service: %w", err)
	}

	jwtService := services.NewJWTService(&cfg.JWT, userRepository, refreshTokenRepository)

	registrationService := services.NewRegistrationService(userRepository, emailService, cfg)
	passwordResetService := services.NewPasswordResetService(userRepository, emailService)
	deletionService := services.NewAccountDeletionService(userRepository, postRepository, deletionRepository, emailService)
	sessionService := services.NewSessionService(jwtService, userRepository)
	userManagementService := services.NewUserManagementService(userRepository, postRepository, emailService)

	authFacade := services.NewAuthFacade(
		registrationService,
		passwordResetService,
		deletionService,
		sessionService,
		userManagementService,
		cfg,
	)

	voteService := services.NewVoteService(voteRepository, postRepository, db)

	voteHandler := handlers.NewVoteHandler(voteService)
	metadataService := services.NewURLMetadataService()

	postHandler := handlers.NewPostHandler(postRepository, metadataService, voteService)
	userHandler := handlers.NewUserHandler(userRepository, authFacade)
	authHandler := handlers.NewAuthHandler(authFacade, userRepository)
	apiHandler := handlers.NewAPIHandlerWithMonitoring(cfg, postRepository, userRepository, voteService, db, dbMonitor)
	pageHandler, err := handlers.NewPageHandler("./internal/templates", authFacade, postRepository, voteService, userRepository, metadataService, cfg)
	if err != nil {
		return fmt.Errorf("load templates: %w", err)
	}

	router := server.NewRouter(server.RouterConfig{
		AuthHandler:     authHandler,
		PostHandler:     postHandler,
		VoteHandler:     voteHandler,
		UserHandler:     userHandler,
		APIHandler:      apiHandler,
		AuthService:     authFacade,
		PageHandler:     pageHandler,
		StaticDir:       "./internal/static/",
		Debug:           cfg.App.Debug,
		DBMonitor:       dbMonitor,
		RateLimitConfig: cfg.RateLimit,
	})

	serverAddr := cfg.Server.Host + ":" + cfg.Server.Port
	log.Printf("Server starting on %s", serverAddr)

	srv := &http.Server{
		Addr:           serverAddr,
		Handler:        router,
		ReadTimeout:    cfg.Server.ReadTimeout,
		WriteTimeout:   cfg.Server.WriteTimeout,
		IdleTimeout:    cfg.Server.IdleTimeout,
		MaxHeaderBytes: cfg.Server.MaxHeaderBytes,
	}

	if cfg.Server.EnableTLS {
		log.Printf("TLS enabled")

		srv.TLSConfig = &tls.Config{
			MinVersion:               tls.VersionTLS12,
			PreferServerCipherSuites: true,
			CipherSuites: []uint16{
				tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
				tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
				tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
				tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,
			},
		}

		return srv.ListenAndServeTLS(cfg.Server.TLSCertFile, cfg.Server.TLSKeyFile)
	}

	log.Printf("WARNING: Server is running on plain HTTP. Enable TLS for production use.")

	return srv.ListenAndServe()
}

var runServer = runServerImpl

func setRunServer(fn func(cfg *config.Config, daemon bool) error) {
	runServer = fn
}
