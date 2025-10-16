.PHONY: help build run test clean docker-build docker-run deps lint fmt vet swagger migrate-up migrate-down security
.DEFAULT_GOAL := help

# Application
APP_NAME := enterprise-vm-manager
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "v0.1.0")
BUILD_TIME := $(shell date -u '+%Y-%m-%d_%H:%M:%S')
GIT_COMMIT := $(shell git rev-parse HEAD 2>/dev/null || echo "unknown")
LDFLAGS := -X main.version=${VERSION} -X main.buildTime=${BUILD_TIME} -X main.gitCommit=${GIT_COMMIT}

# Directories
BUILD_DIR := build
DOCS_DIR := docs
API_DIR := api

# Go parameters
GO_FILES := $(shell find . -type f -name '*.go' -not -path "./vendor/*")
GO_PACKAGES := $(shell go list ./... | grep -v /vendor/)

# Docker
DOCKER_IMAGE := ${APP_NAME}
DOCKER_TAG := ${VERSION}

# Colors for output
RED := \033[31m
GREEN := \033[32m
YELLOW := \033[33m
BLUE := \033[34m
RESET := \033[0m

help: ## Display this help message
	@echo "${GREEN}${APP_NAME} v${VERSION}${RESET}"
	@echo ""
	@echo "${BLUE}Available commands:${RESET}"
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  ${GREEN}%-20s${RESET} %s\n", $$1, $$2}' $(MAKEFILE_LIST)

## Development Commands
build: ## Build the application
	@echo "${BLUE}Building ${APP_NAME}...${RESET}"
	@mkdir -p ${BUILD_DIR}
	@CGO_ENABLED=0 go build -ldflags "${LDFLAGS}" -o ${BUILD_DIR}/${APP_NAME}-server ./cmd/server
	@CGO_ENABLED=0 go build -ldflags "${LDFLAGS}" -o ${BUILD_DIR}/${APP_NAME}-cli ./cmd/cli
	@echo "${GREEN}Build completed successfully${RESET}"

build-linux: ## Build for Linux
	@echo "${BLUE}Building ${APP_NAME} for Linux...${RESET}"
	@mkdir -p ${BUILD_DIR}
	@CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags "${LDFLAGS}" -o ${BUILD_DIR}/${APP_NAME}-server-linux ./cmd/server
	@echo "${GREEN}Linux build completed successfully${RESET}"

run: ## Run the application locally
	@echo "${BLUE}Starting ${APP_NAME}...${RESET}"
	@go run ./cmd/server

run-dev: ## Run with air for hot reloading
	@echo "${BLUE}Starting ${APP_NAME} with hot reload...${RESET}"
	@air

deps: ## Download and install dependencies
	@echo "${BLUE}Downloading dependencies...${RESET}"
	@go mod download
	@go mod tidy
	@go mod verify
	@echo "${GREEN}Dependencies installed successfully${RESET}"

deps-update: ## Update all dependencies
	@echo "${BLUE}Updating dependencies...${RESET}"
	@go get -u ./...
	@go mod tidy
	@echo "${GREEN}Dependencies updated successfully${RESET}"

## Code Quality Commands
fmt: ## Format Go code
	@echo "${BLUE}Formatting code...${RESET}"
	@go fmt ${GO_PACKAGES}
	@goimports -w ${GO_FILES}

vet: ## Run go vet
	@echo "${BLUE}Running go vet...${RESET}"
	@go vet ${GO_PACKAGES}

lint: ## Run golangci-lint
	@echo "${BLUE}Running linter...${RESET}"
	@golangci-lint run

lint-fix: ## Run golangci-lint with fix
	@echo "${BLUE}Running linter with auto-fix...${RESET}"
	@golangci-lint run --fix

security: ## Run gosec security scanner
	@echo "${BLUE}Running security scan...${RESET}"
	@gosec -fmt json -out gosec-report.json -exclude-generated ./...
	@gosec ./...

## Testing Commands
test: ## Run tests
	@echo "${BLUE}Running tests...${RESET}"
	@go test -v -race -coverprofile=coverage.out ${GO_PACKAGES}

test-unit: ## Run unit tests only
	@echo "${BLUE}Running unit tests...${RESET}"
	@go test -v -race -short ${GO_PACKAGES}

test-integration: ## Run integration tests only
	@echo "${BLUE}Running integration tests...${RESET}"
	@go test -v -race -run Integration ${GO_PACKAGES}

test-coverage: ## Run tests with coverage report
	@echo "${BLUE}Running tests with coverage...${RESET}"
	@go test -v -race -coverprofile=coverage.out -covermode=atomic ${GO_PACKAGES}
	@go tool cover -html=coverage.out -o coverage.html
	@echo "${GREEN}Coverage report generated: coverage.html${RESET}"

benchmark: ## Run benchmarks
	@echo "${BLUE}Running benchmarks...${RESET}"
	@go test -bench=. -benchmem ${GO_PACKAGES}

## Database Commands
migrate-up: ## Run database migrations up
	@echo "${BLUE}Running database migrations up...${RESET}"
	@migrate -path internal/database/migrations -database "$${DATABASE_URL}" up

migrate-down: ## Run database migrations down
	@echo "${BLUE}Running database migrations down...${RESET}"
	@migrate -path internal/database/migrations -database "$${DATABASE_URL}" down

migrate-force: ## Force migration version
	@echo "${BLUE}Forcing migration version...${RESET}"
	@migrate -path internal/database/migrations -database "$${DATABASE_URL}" force $(VERSION)

migrate-create: ## Create new migration (usage: make migrate-create NAME=create_users_table)
	@echo "${BLUE}Creating migration: ${NAME}...${RESET}"
	@migrate create -ext sql -dir internal/database/migrations -seq ${NAME}

## Docker Commands
docker-build: ## Build Docker image
	@echo "${BLUE}Building Docker image...${RESET}"
	@docker build -t ${DOCKER_IMAGE}:${DOCKER_TAG} -t ${DOCKER_IMAGE}:latest .
	@echo "${GREEN}Docker image built: ${DOCKER_IMAGE}:${DOCKER_TAG}${RESET}"

docker-run: ## Run with Docker Compose
	@echo "${BLUE}Starting services with Docker Compose...${RESET}"
	@docker-compose up -d

docker-stop: ## Stop Docker Compose services
	@echo "${BLUE}Stopping Docker services...${RESET}"
	@docker-compose down

docker-logs: ## Show Docker logs
	@docker-compose logs -f api

docker-clean: ## Clean Docker resources
	@echo "${BLUE}Cleaning Docker resources...${RESET}"
	@docker-compose down -v --remove-orphans
	@docker system prune -f

## Documentation Commands
swagger: ## Generate Swagger documentation
	@echo "${BLUE}Generating Swagger documentation...${RESET}"
	@swag init -g ./cmd/server/main.go -o ./api/openapi --parseDependency --parseInternal
	@echo "${GREEN}Swagger documentation generated${RESET}"

docs: ## Generate all documentation
	@echo "${BLUE}Generating documentation...${RESET}"
	@make swagger
	@echo "${GREEN}Documentation generated successfully${RESET}"

## Deployment Commands
k8s-deploy: ## Deploy to Kubernetes
	@echo "${BLUE}Deploying to Kubernetes...${RESET}"
	@kubectl apply -f deployments/kubernetes/

k8s-delete: ## Delete from Kubernetes  
	@echo "${BLUE}Deleting from Kubernetes...${RESET}"
	@kubectl delete -f deployments/kubernetes/

helm-install: ## Install Helm chart
	@echo "${BLUE}Installing Helm chart...${RESET}"
	@helm install vm-manager deployments/helm/vm-manager/

helm-upgrade: ## Upgrade Helm chart
	@echo "${BLUE}Upgrading Helm chart...${RESET}"
	@helm upgrade vm-manager deployments/helm/vm-manager/

helm-uninstall: ## Uninstall Helm chart
	@echo "${BLUE}Uninstalling Helm chart...${RESET}"
	@helm uninstall vm-manager

## Utility Commands
clean: ## Clean build artifacts
	@echo "${BLUE}Cleaning build artifacts...${RESET}"
	@rm -rf ${BUILD_DIR}
	@rm -f coverage.out coverage.html
	@rm -f gosec-report.json
	@go clean -cache -testcache -modcache
	@echo "${GREEN}Clean completed${RESET}"

install-tools: ## Install development tools
	@echo "${BLUE}Installing development tools...${RESET}"
	@go install github.com/cosmtrek/air@latest
	@go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	@go install github.com/securecodewarrior/gosec/v2/cmd/gosec@latest
	@go install github.com/swaggo/swag/cmd/swag@latest
	@go install golang.org/x/tools/cmd/goimports@latest
	@go install github.com/golang-migrate/migrate/v4/cmd/migrate@latest
	@echo "${GREEN}Development tools installed${RESET}"

version: ## Show version information
	@echo "${GREEN}${APP_NAME}${RESET}"
	@echo "Version: ${VERSION}"
	@echo "Build Time: ${BUILD_TIME}"
	@echo "Git Commit: ${GIT_COMMIT}"

check: fmt vet lint security test ## Run all checks (format, vet, lint, security, test)

ci: deps check build ## Run CI pipeline

release: clean check build-linux docker-build ## Build release artifacts

## Environment Commands
env-dev: ## Set up development environment
	@echo "${BLUE}Setting up development environment...${RESET}"
	@cp configs/config.example.yaml configs/config.dev.yaml
	@cp .env.example .env
	@docker-compose up -d postgres redis
	@echo "${GREEN}Development environment ready${RESET}"

env-test: ## Set up test environment
	@echo "${BLUE}Setting up test environment...${RESET}"
	@docker-compose -f docker-compose.test.yml up -d
	@echo "${GREEN}Test environment ready${RESET}"
