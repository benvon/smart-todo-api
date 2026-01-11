#!/bin/bash

# Setup script for AI Code Template Go
# This script helps set up the development environment

set -e

# Colors for output
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

echo -e "${GREEN}Setting up AI Code Template Go development environment...${NC}"

# Check if Go is installed
if ! command -v go &> /dev/null; then
    echo -e "${RED}Error: Go is not installed. Please install Go 1.23 or later.${NC}"
    echo -e "${YELLOW}Visit: https://golang.org/doc/install${NC}"
    exit 1
fi

# Check Go version
GO_VERSION=$(go version | awk '{print $3}' | sed 's/go//')
echo -e "${GREEN}Found Go version: ${GO_VERSION}${NC}"

# Check if required tools are installed
echo -e "${YELLOW}Checking required development tools...${NC}"

# Install golangci-lint if not present
if ! command -v golangci-lint &> /dev/null; then
    echo -e "${YELLOW}Installing golangci-lint...${NC}"
    curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(go env GOPATH)/bin latest
else
    echo -e "${GREEN}golangci-lint is already installed${NC}"
fi

# Install govulncheck if not present
if ! command -v govulncheck &> /dev/null; then
    echo -e "${YELLOW}Installing govulncheck...${NC}"
    go install golang.org/x/vuln/cmd/govulncheck@latest
else
    echo -e "${GREEN}govulncheck is already installed${NC}"
fi

# Install gosec if not present
if ! command -v gosec &> /dev/null; then
    echo -e "${YELLOW}Installing gosec...${NC}"
    go install github.com/securecodewarrior/gosec/v2/cmd/gosec@latest
else
    echo -e "${GREEN}gosec is already installed${NC}"
fi

# Install pre-commit if not present
if ! command -v pre-commit &> /dev/null; then
    echo -e "${YELLOW}Installing pre-commit...${NC}"
    if command -v pip3 &> /dev/null; then
        pip3 install pre-commit
    elif command -v pip &> /dev/null; then
        pip install pre-commit
    else
        echo -e "${YELLOW}Note: pip not found. Please install pre-commit manually.${NC}"
        echo -e "${YELLOW}Visit: https://pre-commit.com/#install${NC}"
    fi
else
    echo -e "${GREEN}pre-commit is already installed${NC}"
fi

# Download Go dependencies
echo -e "${YELLOW}Downloading Go dependencies...${NC}"
go mod download
go mod verify

# Install pre-commit hooks
if command -v pre-commit &> /dev/null; then
    echo -e "${YELLOW}Installing pre-commit hooks...${NC}"
    pre-commit install
else
    echo -e "${YELLOW}Skipping pre-commit hooks installation (pre-commit not found)${NC}"
fi

# Create .env file from example if it doesn't exist
if [ ! -f .env ]; then
    if [ -f .env.example ]; then
        echo -e "${YELLOW}Creating .env file from .env.example...${NC}"
        cp .env.example .env
        echo -e "${YELLOW}Please review and update .env with your configuration${NC}"
    fi
fi

# Run initial tests
echo -e "${YELLOW}Running initial tests...${NC}"
go test ./...

# Run initial linting
echo -e "${YELLOW}Running initial linting...${NC}"
if command -v golangci-lint &> /dev/null; then
    golangci-lint run
else
    echo -e "${YELLOW}Skipping linting (golangci-lint not found)${NC}"
fi

echo -e "${GREEN}Setup completed successfully!${NC}"
echo -e "${YELLOW}Next steps:${NC}"
echo -e "  1. Review and update .env file with your configuration"
echo -e "  2. Run 'make help' to see available commands"
echo -e "  3. Run 'make test' to run tests"
echo -e "  4. Run 'make build' to build the application"
echo -e "  5. Run 'make docker-build' to build Docker image"
echo -e "  6. Start developing your application!"
