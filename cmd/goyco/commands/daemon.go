package commands

import (
	"errors"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"sync"
	"syscall"
	"time"

	"goyco/internal/config"
)

func HandleStartCommand(cfg *config.Config, args []string) error {
	fs := newFlagSet("start", printStartUsage)
	if err := parseCommand(fs, args, "start"); err != nil {
		if errors.Is(err, ErrHelpRequested) {
			return nil
		}
		return err
	}

	if fs.NArg() > 0 {
		printStartUsage()
		return errors.New("unexpected arguments for start command")
	}

	return runDaemon(cfg)
}

func HandleStopCommand(cfg *config.Config, args []string) error {
	fs := newFlagSet("stop", printStopUsage)
	if err := parseCommand(fs, args, "stop"); err != nil {
		if errors.Is(err, ErrHelpRequested) {
			return nil
		}
		return err
	}

	if fs.NArg() > 0 {
		printStopUsage()
		return errors.New("unexpected arguments for stop command")
	}

	return stopDaemon(cfg)
}

func HandleStatusCommand(cfg *config.Config, name string, args []string) error {
	fs := newFlagSet(name, printStatusUsage)
	if err := parseCommand(fs, args, name); err != nil {
		if errors.Is(err, ErrHelpRequested) {
			return nil
		}
		return err
	}

	if fs.NArg() > 0 {
		printStatusUsage()
		return errors.New("unexpected arguments for status command")
	}

	return runStatusCommand(cfg)
}

func printStartUsage() {
	fmt.Fprintln(os.Stderr, "Usage: goyco start")
	fmt.Fprintln(os.Stderr, "\nStart the web application in background.")
}

func printStopUsage() {
	fmt.Fprintln(os.Stderr, "Usage: goyco stop")
	fmt.Fprintln(os.Stderr, "\nStop the running daemon.")
}

func printStatusUsage() {
	fmt.Fprintln(os.Stderr, "Usage: goyco status")
	fmt.Fprintln(os.Stderr, "\nCheck if the daemon is running.")
}

func runStatusCommand(cfg *config.Config) error {
	pidDir := cfg.PIDDir
	pidFile := filepath.Join(pidDir, "goyco.pid")

	if !isDaemonRunning(pidFile) {
		if IsJSONOutput() {
			outputJSON(map[string]any{
				"status": "not_running",
			})
		} else {
			fmt.Println("Goyco is not running")
		}
		return nil
	}

	data, err := os.ReadFile(pidFile)
	if err != nil {
		if IsJSONOutput() {
			outputJSON(map[string]any{
				"status": "running",
				"error":  fmt.Sprintf("PID file exists but cannot be read: %v", err),
			})
		} else {
			fmt.Printf("Goyco is running (PID file exists but cannot be read: %v)\n", err)
		}
		return nil
	}

	pid, err := strconv.Atoi(string(data))
	if err != nil {
		if IsJSONOutput() {
			outputJSON(map[string]any{
				"status": "running",
				"error":  fmt.Sprintf("PID file exists but contains invalid PID: %v", err),
			})
		} else {
			fmt.Printf("Goyco is running (PID file exists but contains invalid PID: %v)\n", err)
		}
		return nil
	}

	if IsJSONOutput() {
		outputJSON(map[string]any{
			"status": "running",
			"pid":    pid,
		})
	} else {
		fmt.Printf("Goyco is running (PID %d)\n", pid)
	}
	return nil
}

func stopDaemon(cfg *config.Config) error {
	pidDir := cfg.PIDDir
	pidFile := filepath.Join(pidDir, "goyco.pid")

	if !isDaemonRunning(pidFile) {
		return fmt.Errorf("daemon is not running")
	}

	data, err := os.ReadFile(pidFile)
	if err != nil {
		return fmt.Errorf("read PID file: %w", err)
	}

	pid, err := strconv.Atoi(string(data))
	if err != nil {
		return fmt.Errorf("parse PID: %w", err)
	}

	process, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("find process: %w", err)
	}

	if err := process.Signal(syscall.SIGTERM); err != nil {
		return fmt.Errorf("send SIGTERM: %w", err)
	}

	time.Sleep(2 * time.Second)

	if isDaemonRunning(pidFile) {
		if err := process.Signal(syscall.SIGKILL); err != nil {
			return fmt.Errorf("send SIGKILL: %w", err)
		}
	}

	_ = os.Remove(pidFile)

	if IsJSONOutput() {
		outputJSON(map[string]any{
			"action": "stopped",
			"pid":    pid,
		})
	} else {
		fmt.Printf("Goyco stopped (PID %d)\n", pid)
	}
	return nil
}

func runDaemon(cfg *config.Config) error {
	logDir := cfg.LogDir
	if logDir == "" {
		logDir = "/var/log"
	}

	if err := os.MkdirAll(logDir, 0o755); err != nil {
		return fmt.Errorf("create log directory: %w", err)
	}

	pidDir := cfg.PIDDir
	if pidDir == "" {
		pidDir = "/run"
	}
	if err := os.MkdirAll(pidDir, 0o755); err != nil {
		return fmt.Errorf("create PID directory: %w", err)
	}

	pidFile := filepath.Join(pidDir, "goyco.pid")
	logFile := filepath.Join(logDir, "goyco.log")

	if isDaemonRunning(pidFile) {
		return fmt.Errorf("daemon is already running (PID file exists: %s)", pidFile)
	}

	daemonizeFnMu.Lock()
	fn := daemonizeFn
	daemonizeFnMu.Unlock()
	pid, err := fn()
	if err != nil {
		return fmt.Errorf("failed to daemonize: %w", err)
	}

	if pid > 0 {
		if err := writePIDFile(pidFile, pid); err != nil {
			return fmt.Errorf("cannot write PID file: %w", err)
		}
		if IsJSONOutput() {
			outputJSON(map[string]any{
				"action":   "started",
				"pid":      pid,
				"pid_file": pidFile,
				"log_file": logFile,
			})
		} else {
			fmt.Printf("Goyco started with PID %d\n", pid)
			fmt.Printf("PID file: %s\n", pidFile)
			fmt.Printf("Log file: %s\n", logFile)
		}
		return nil
	}

	return runDaemonProcess(cfg, logDir, pidFile)
}

func daemonizeImpl() (int, error) {
	args := make([]string, len(os.Args))
	copy(args, os.Args)
	args = append(args, "--daemon")

	pid, err := syscall.ForkExec(os.Args[0], args, &syscall.ProcAttr{
		Files: []uintptr{0, 1, 2},
		Env:   os.Environ(),
	})
	if err != nil {
		return 0, err
	}

	return pid, nil
}

func isDaemonRunning(pidFile string) bool {
	if _, err := os.Stat(pidFile); os.IsNotExist(err) {
		return false
	}

	data, err := os.ReadFile(pidFile)
	if err != nil {
		return false
	}

	pid, err := strconv.Atoi(string(data))
	if err != nil {
		return false
	}

	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}

	err = process.Signal(syscall.Signal(0))
	return err == nil
}

func writePIDFile(pidFile string, pid int) error {
	return os.WriteFile(pidFile, []byte(strconv.Itoa(pid)), 0o644)
}

func runDaemonProcess(cfg *config.Config, logDir, pidFile string) error {
	daemonizeFnMu.Lock()
	setupLogFn := setupLoggingFn
	daemonizeFnMu.Unlock()
	if err := setupLogFn(cfg, logDir); err != nil {
		return fmt.Errorf("setup daemon logging: %w", err)
	}

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGTERM, syscall.SIGINT)

	serverErr := make(chan error, 1)
	go func() {
		serverErr <- runServer(cfg, true)
	}()

	select {
	case sig := <-sigChan:
		log.Printf("Received signal %v, shutting down gracefully...", sig)
		if err := os.Remove(pidFile); err != nil {
			log.Printf("Error removing PID file: %v", err)
		}
		return nil
	case err := <-serverErr:
		if removeErr := os.Remove(pidFile); removeErr != nil {
			log.Printf("Error removing PID file: %v", removeErr)
		}
		return err
	}
}

func setupDaemonLoggingImpl(cfg *config.Config, logDir string) error {
	logFile := filepath.Join(logDir, "goyco.log")

	logFileHandle, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return fmt.Errorf("open log file: %w", err)
	}

	log.SetOutput(logFileHandle)
	log.SetFlags(log.LstdFlags)

	log.Printf("Starting goyco in daemon mode")

	return nil
}

func SetupDaemonLogging(cfg *config.Config, logDir string) error {
	daemonizeFnMu.Lock()
	setupLogFn := setupLoggingFn
	daemonizeFnMu.Unlock()
	return setupLogFn(cfg, logDir)
}

var runServer func(cfg *config.Config, daemon bool) error

func SetRunServer(fn func(cfg *config.Config, daemon bool) error) {
	runServer = fn
}

type daemonizeFunc func() (int, error)

var (
	daemonizeFnMu  sync.Mutex
	daemonizeFn    daemonizeFunc                                 = daemonizeImpl
	setupLoggingFn func(cfg *config.Config, logDir string) error = setupDaemonLoggingImpl
)

func SetDaemonize(fn daemonizeFunc) {
	daemonizeFnMu.Lock()
	defer daemonizeFnMu.Unlock()
	if fn == nil {
		daemonizeFn = daemonizeImpl
	} else {
		daemonizeFn = fn
	}
}

func SetSetupDaemonLogging(fn func(cfg *config.Config, logDir string) error) {
	daemonizeFnMu.Lock()
	defer daemonizeFnMu.Unlock()
	if fn == nil {
		setupLoggingFn = setupDaemonLoggingImpl
	} else {
		setupLoggingFn = fn
	}
}

func RunDaemonProcessDirect(_ []string) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load configuration: %w", err)
	}

	logDir := cfg.LogDir
	if logDir == "" {
		return fmt.Errorf("LOG_DIR environment variable is required for daemon mode")
	}

	pidDir := cfg.PIDDir
	if err := os.MkdirAll(pidDir, 0o755); err != nil {
		return fmt.Errorf("create PID directory: %w", err)
	}

	pidFile := filepath.Join(pidDir, "goyco.pid")
	return runDaemonProcess(cfg, logDir, pidFile)
}
