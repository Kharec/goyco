#!/bin/bash
# regenerate swagger
set -e

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

echo -e "${BLUE}Regenerating Swagger documentation for Goyco API...${NC}"
cd "$PROJECT_ROOT"

if ! command -v swag &>/dev/null; then
  echo -e "${RED}Error: 'swag' command not found. Please install it first:${NC}"
  echo -e "${YELLOW}   go install github.com/swaggo/swag/cmd/swag@latest${NC}"
  exit 1
fi

if [ ! -f "cmd/goyco/main.go" ]; then
  echo -e "${RED}Error: cmd/goyco/main.go not found${NC}"
  exit 1
fi

if [ ! -d "docs" ]; then
  echo -e "${YELLOW}Creating docs directory...${NC}"
  mkdir -p docs
fi

SWAGGER_DIRECTORIES="cmd/goyco,internal/handlers,internal/dto"
SWAGGER_MAIN_FILE="main.go"
SWAGGER_OUTPUT_DIR="docs"

echo -e "${BLUE}Running swag init on directories: ${SWAGGER_DIRECTORIES}...${NC}"

if swag init -g "$SWAGGER_MAIN_FILE" -d "$SWAGGER_DIRECTORIES" -o "$SWAGGER_OUTPUT_DIR"; then
  echo -e "${GREEN}Swagger documentation generated successfully!${NC}"
  echo -e "${BLUE}Generated files:${NC}"
  ls -la docs/swagger.* docs/docs.go 2>/dev/null || true

  if [ -f "docs/swagger.json" ]; then
    ENDPOINT_COUNT=$(grep -o '"/[^"]*":' docs/swagger.json | wc -l)
    echo -e "${GREEN}Found $ENDPOINT_COUNT API endpoints${NC}"
  fi

  if [ -f ".env" ]; then
    export "$(grep -E '^APP_BASE_URL=' .env | xargs)"
  fi

  if [ -n "$APP_BASE_URL" ]; then
    DOCS_URL="${APP_BASE_URL%/}/swagger/index.html"
  else
    DOCS_URL="http://localhost:8080/swagger/index.html"
  fi

  echo -e "${GREEN}You can now view the documentation at: $DOCS_URL${NC}"
  echo -e "${BLUE}To serve the documentation, please start Goyco.${NC}"
else
  echo -e "${RED}Error: Failed to generate Swagger documentation${NC}"
  echo -e "${YELLOW}Make sure you have proper Swagger annotations in your handlers${NC}"
  echo -e "${YELLOW}Check the output above for specific error messages${NC}"
  exit 1
fi

echo -e "${GREEN}Swagger documentation regeneration complete!${NC}"
