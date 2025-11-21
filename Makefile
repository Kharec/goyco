GO ?= go
DOCKER ?= docker
PRETTIER ?= prettier
GOLANGCI_LINT ?= golangci-lint

BINARY := bin/goyco
INSTALL_DIR := /opt/goyco
DOC_DIR := /usr/share/doc/goyco
LICENSE_DIR := /usr/share/licenses/goyco
SERVICE_FILE := /etc/systemd/system/goyco.service

VERSION_FILE := internal/version/version.go
VERSION := $(shell sed -n 's/^const Version = "\(.*\)"/\1/p' $(VERSION_FILE))
DIST_DIR ?= dist
RELEASE_NAME := goyco-$(VERSION)
RELEASE_TARBALL := $(DIST_DIR)/$(RELEASE_NAME).tar.gz
RELEASE_ARCHIVE := $(DIST_DIR)/$(RELEASE_NAME).tar
GO_BUILD_FLAGS ?=
GO_TEST_FLAGS ?= -v 
FUZZ_TIME ?= 30s
DOCKER_IMAGE ?= goyco:latest
SWAGGER_SCRIPT := ./scripts/regenerate-swagger.sh
DEPENDENCY_COMPOSE_FILE := docker/compose.dependencies.yml

UNIT_TEST_PACKAGES := ./cmd/goyco ./internal/config ./internal/database ./internal/fuzz ./internal/handlers ./internal/middleware ./internal/repositories ./internal/security ./internal/server ./internal/services ./internal/templates ./internal/validation
INTEGRATION_TEST_PACKAGE := ./internal/integration/...
E2E_TEST_PACKAGE := ./internal/e2e/...

FUZZ_PACKAGES := ./internal/validation ./internal/security ./internal/handlers ./cmd/goyco ./internal/fuzz

.PHONY: build test clean format lint build-deps clean-deps docker-image swagger \
	unit-tests integration-tests e2e-tests fuzz-tests install uninstall release migrations

build:
	@mkdir -p $(dir $(BINARY))
	$(GO) build $(GO_BUILD_FLAGS) -o $(BINARY) ./cmd/goyco

test: unit-tests integration-tests e2e-tests

clean:
	rm -f $(BINARY)
	$(GO) clean -testcache
	rm -rf .gocache
	rm -fr dist/*

format:
	$(PRETTIER) -w . --ignore-path .prettierignore
	$(GO) fmt $(shell $(GO) list ./... | grep -v 'docs')

lint:
	$(GOLANGCI_LINT) run

build-deps:
	$(DOCKER) compose -f $(DEPENDENCY_COMPOSE_FILE) up -d

clean-deps:
	$(DOCKER) compose -f $(DEPENDENCY_COMPOSE_FILE) down --volumes --remove-orphans

docker-image:
	$(DOCKER) build -t $(DOCKER_IMAGE) -f Dockerfile .

swagger:
	@echo "Regenerating Swagger documentation..."
	@$(SWAGGER_SCRIPT)

unit-tests:
	$(GO) test $(GO_TEST_FLAGS) $(UNIT_TEST_PACKAGES)

integration-tests:
	$(GO) test $(GO_TEST_FLAGS) $(INTEGRATION_TEST_PACKAGE)

e2e-tests:
	$(GO) test $(GO_TEST_FLAGS) $(E2E_TEST_PACKAGE)

fuzz-tests:
	@echo "Running fuzz tests..."
	@set -e; \
	for pkg in $(FUZZ_PACKAGES); do \
		echo "==> Fuzzing $$pkg"; \
		fuzz_targets="$$( $(GO) test -run ^$$ -list ^Fuzz $$pkg | grep '^Fuzz' || true )"; \
		if [ -z "$$fuzz_targets" ]; then \
			echo "No fuzz tests found in $$pkg"; \
			continue; \
		fi; \
		for fuzz in $$fuzz_targets; do \
			echo "   -> $$fuzz"; \
			$(GO) test -run ^$$ -fuzz="^$$fuzz$$" -fuzztime=$(FUZZ_TIME) $$pkg; \
		done; \
	done

install:
	@useradd -r -m -d $(INSTALL_DIR) -s /usr/sbin/nologin goyco	
	@mkdir -p $(INSTALL_DIR)/bin $(INSTALL_DIR)/internal/static $(INSTALL_DIR)/internal/templates /usr/share/licenses/goyco /usr/share/doc/goyco
	@cp $(BINARY) $(INSTALL_DIR)/bin/goyco
	@cp .env.example $(INSTALL_DIR)/.env
	@cp -r internal/static $(INSTALL_DIR)/internal/
	@cp -r internal/templates $(INSTALL_DIR)/internal/
	@cp LICENSE $(LICENSE_DIR)/
	@cp README.md $(DOC_DIR)/
	@cp services/goyco.service $(SERVICE_FILE)

uninstall:
	@systemctl disable --now goyco
	@rm -f $(SERVICE_FILE)
	@rm -rf $(INSTALL_DIR) $(DOC_DIR) $(LICENSE_DIR)
	@userdel goyco

release:
	@test -n "$(VERSION)" || (echo "Version not found in $(VERSION_FILE)" >&2 && exit 1)
	@mkdir -p $(DIST_DIR)
	@rm -f $(RELEASE_TARBALL) $(RELEASE_ARCHIVE)
	@set -e; \
	if git rev-parse --is-inside-work-tree >/dev/null 2>&1; then \
		git archive --format=tar --prefix=$(RELEASE_NAME)/ --output=$(RELEASE_ARCHIVE) HEAD ":!TODO.md" ":!.env"; \
	else \
		tar -cf $(RELEASE_ARCHIVE) --exclude='./$(DIST_DIR)' --exclude='./.git' --exclude='./TODO.md' --exclude='./.env' --transform='s,^./,$(RELEASE_NAME)/,' .; \
	fi
	@gzip -f $(RELEASE_ARCHIVE)
	@echo "Created $(RELEASE_TARBALL)"

migrations:
	@$(INSTALL_DIR)/bin/goyco migrate
