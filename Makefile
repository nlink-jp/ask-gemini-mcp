BINARY  := ask-gemini-mcp
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS := -ldflags "-X main.version=$(VERSION)"
DIST_DIR := dist

.PHONY: build build-all test test-e2e clean install

build:
	@mkdir -p $(DIST_DIR)
	go build $(LDFLAGS) -o $(DIST_DIR)/$(BINARY) .

build-all:
	@mkdir -p $(DIST_DIR)
	CGO_ENABLED=0 GOOS=linux   GOARCH=amd64 go build $(LDFLAGS) -o $(DIST_DIR)/$(BINARY)-linux-amd64   .
	CGO_ENABLED=0 GOOS=linux   GOARCH=arm64 go build $(LDFLAGS) -o $(DIST_DIR)/$(BINARY)-linux-arm64   .
	CGO_ENABLED=0 GOOS=darwin  GOARCH=amd64 go build $(LDFLAGS) -o $(DIST_DIR)/$(BINARY)-darwin-amd64  .
	CGO_ENABLED=0 GOOS=darwin  GOARCH=arm64 go build $(LDFLAGS) -o $(DIST_DIR)/$(BINARY)-darwin-arm64  .
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o $(DIST_DIR)/$(BINARY)-windows-amd64.exe .

test:
	go test ./...

# E2E tests spawn the built binary and drive it over stdio. They make real
# Vertex AI calls, so set ASK_GEMINI_PROJECT and run `gcloud auth
# application-default login` before invoking. Build first; the harness
# reads ASK_GEMINI_TEST_BINARY (defaults to ./dist/ask-gemini-mcp).
test-e2e: build
	ASK_GEMINI_TEST_BINARY=$$(pwd)/$(DIST_DIR)/$(BINARY) go test -tags e2e ./e2e/...

clean:
	rm -rf $(DIST_DIR)

install: build
	install -m 0755 $(DIST_DIR)/$(BINARY) $(HOME)/bin/$(BINARY)
