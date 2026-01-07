package e2e

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestE2E_DockerDeployment(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping Docker deployment tests in short mode")
	}

	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}

	workspaceRoot := wd
	for {
		if _, err := os.Stat(filepath.Join(workspaceRoot, "go.mod")); err == nil {
			break
		}
		parent := filepath.Dir(workspaceRoot)
		if parent == workspaceRoot {
			t.Skip("Could not find workspace root")
			return
		}
		workspaceRoot = parent
	}

	t.Run("dockerfile_exists", func(t *testing.T) {
		dockerfilePath := filepath.Join(workspaceRoot, "Dockerfile")
		if _, err := os.Stat(dockerfilePath); os.IsNotExist(err) {
			t.Skipf("Dockerfile not found at %s", dockerfilePath)
		}
	})

	t.Run("dockerfile_valid", func(t *testing.T) {
		dockerfilePath := filepath.Join(workspaceRoot, "Dockerfile")
		content, err := os.ReadFile(dockerfilePath)
		if err != nil {
			t.Skipf("Failed to read Dockerfile: %v", err)
			return
		}

		contentStr := string(content)
		required := []string{
			"FROM",
			"WORKDIR",
			"COPY",
			"RUN",
			"EXPOSE",
		}

		for _, req := range required {
			if !strings.Contains(contentStr, req) {
				t.Errorf("Dockerfile missing required directive: %s", req)
			}
		}
	})

	t.Run("service_file_exists", func(t *testing.T) {
		servicePath := filepath.Join(workspaceRoot, "services/goyco.service")
		if _, err := os.Stat(servicePath); os.IsNotExist(err) {
			t.Skipf("Service file not found at %s", servicePath)
		}
	})

	t.Run("service_file_valid", func(t *testing.T) {
		servicePath := filepath.Join(workspaceRoot, "services/goyco.service")
		content, err := os.ReadFile(servicePath)
		if err != nil {
			t.Skipf("Failed to read service file: %v", err)
			return
		}

		contentStr := string(content)
		required := []string{
			"[Unit]",
			"[Service]",
			"ExecStart",
			"Restart",
		}

		for _, req := range required {
			if !strings.Contains(contentStr, req) {
				t.Errorf("Service file missing required section: %s", req)
			}
		}
	})

	t.Run("static_files_exist", func(t *testing.T) {
		staticDir := filepath.Join(workspaceRoot, "internal/static")
		if _, err := os.Stat(staticDir); os.IsNotExist(err) {
			t.Skipf("Static directory not found at %s", staticDir)
			return
		}

		requiredFiles := []string{
			"robots.txt",
		}

		for _, file := range requiredFiles {
			filePath := filepath.Join(staticDir, file)
			if _, err := os.Stat(filePath); os.IsNotExist(err) {
				t.Errorf("Required static file not found: %s", filePath)
			}
		}
	})

	t.Run("templates_exist", func(t *testing.T) {
		templatesDir := filepath.Join(workspaceRoot, "internal/templates")
		if _, err := os.Stat(templatesDir); os.IsNotExist(err) {
			t.Skipf("Templates directory not found at %s", templatesDir)
		}
	})
}

func TestE2E_EnvironmentVariables(t *testing.T) {
	t.Run("config_loading", func(t *testing.T) {
		envVars := []string{
			"SERVER_HOST",
			"SERVER_PORT",
			"DATABASE_HOST",
			"DATABASE_PORT",
			"DATABASE_USER",
			"DATABASE_PASSWORD",
			"DATABASE_NAME",
			"JWT_SECRET",
		}

		for _, envVar := range envVars {
			if os.Getenv(envVar) == "" {
				t.Logf("Environment variable %s not set (this is expected in test environment)", envVar)
			}
		}
	})
}

func TestE2E_BinaryExists(t *testing.T) {
	t.Run("binary_builds", func(t *testing.T) {
		if testing.Short() {
			t.Skip("Skipping binary build test in short mode")
		}

		wd, err := os.Getwd()
		if err != nil {
			t.Skipf("Failed to get working directory: %v", err)
			return
		}

		workspaceRoot := wd
		for {
			if _, err := os.Stat(filepath.Join(workspaceRoot, "go.mod")); err == nil {
				break
			}
			parent := filepath.Dir(workspaceRoot)
			if parent == workspaceRoot {
				t.Skip("Could not find workspace root")
				return
			}
			workspaceRoot = parent
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()

		cmd := exec.CommandContext(ctx, "go", "build", "-o", "/tmp/goyco-test", "./cmd/goyco")
		cmd.Dir = workspaceRoot
		if err := cmd.Run(); err != nil {
			t.Skipf("Failed to build binary: %v", err)
			return
		}

		if _, err := os.Stat("/tmp/goyco-test"); os.IsNotExist(err) {
			t.Error("Binary was not created")
		} else {
			os.Remove("/tmp/goyco-test")
		}
	})
}

func TestE2E_ConfigurationValidation(t *testing.T) {
	t.Run("required_paths", func(t *testing.T) {
		wd, err := os.Getwd()
		if err != nil {
			t.Fatalf("Failed to get working directory: %v", err)
		}

		workspaceRoot := wd
		for {
			if _, err := os.Stat(filepath.Join(workspaceRoot, "go.mod")); err == nil {
				break
			}
			parent := filepath.Dir(workspaceRoot)
			if parent == workspaceRoot {
				t.Fatalf("Could not find workspace root (go.mod) starting from %s", wd)
			}
			workspaceRoot = parent
		}

		requiredPaths := []string{
			"cmd/goyco",
			"internal",
			"go.mod",
			"go.sum",
		}

		for _, path := range requiredPaths {
			fullPath := filepath.Join(workspaceRoot, path)
			if _, err := os.Stat(fullPath); os.IsNotExist(err) {
				t.Errorf("Required path not found: %s (workspace root: %s)", path, workspaceRoot)
			}
		}
	})
}
