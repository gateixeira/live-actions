.PHONY: build build-frontend run test clean docker-build docker-run lint fmt fmt-imports vet check test-coverage clean-coverage all

# Go related variables
BINARY_NAME=live-actions
MAIN_PACKAGE=.

# Build variables
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
DATE ?= $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")

# Go build flags
LDFLAGS = -ldflags "-X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE)"

# Go commands
GOCMD=go
GOBUILD=$(GOCMD) build
GORUN=$(GOCMD) run
GOTEST=$(GOCMD) test
GOCLEAN=$(GOCMD) clean
GOGET=$(GOCMD) get
GOLINT=golangci-lint

# Build the frontend React app
build-frontend:
	cd frontend && npm install && npm run build

# Build the application
build: build-frontend
	$(GOBUILD) $(LDFLAGS) -o $(BINARY_NAME) $(MAIN_PACKAGE)

# Run the application
run:
	$(GORUN) $(MAIN_PACKAGE)

# Run tests
test:
	$(GOTEST) ./...

# Clean build files
clean:
	$(GOCLEAN)
	rm -f $(BINARY_NAME)

# Build docker image
docker-build:
	docker build -t $(BINARY_NAME) .

# Run docker container
docker-run: docker-build
	docker run -p 8080:8080 \
		-e WEBHOOK_SECRET=$${WEBHOOK_SECRET} \
		-v live-actions-data:/app/data \
		$(BINARY_NAME)

# Install dependencies
deps:
	$(GOGET) -v ./...

fmt:
	go fmt ./...

fmt-imports:
	goimports -w .

lint:
	$(GOLINT) run

vet:
	go vet ./...

# Run format, lint, vet, and test
check: fmt lint vet
	$(GOTEST) ./...

test-coverage:
	$(GOTEST) -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

# Clean coverage files
clean-coverage:
	rm -f coverage.out coverage.html

# Enhanced clean target
clean: clean-coverage
	$(GOCLEAN)
	rm -f $(BINARY_NAME)

# Default target
all: clean build