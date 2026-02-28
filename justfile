# Default recipe
default: build

# Build all binaries
build:
    go build ./cmd/kapso-whatsapp-cli
    go build ./cmd/kapso-whatsapp-poller

# Run all tests
test:
    go test ./...

# Run tests with verbose output
test-v:
    go test -v ./...

# Run linter (requires golangci-lint)
lint:
    golangci-lint run ./...

# Format code
fmt:
    gofmt -w .

# Check formatting (CI-friendly, fails if unformatted)
fmt-check:
    test -z "$(gofmt -l .)"

# Vet code
vet:
    go vet ./...

# Install binaries to $GOPATH/bin
install:
    go install ./cmd/kapso-whatsapp-cli
    go install ./cmd/kapso-whatsapp-poller

# Clean build artifacts
clean:
    rm -f kapso-whatsapp-cli kapso-whatsapp-poller

# Run all checks (test + vet + fmt)
check: test vet fmt-check
