APP := vps-watchdog
VERSION ?= dev
COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
DATE := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
DIST_DIR := dist
BIN := $(DIST_DIR)/$(APP)-linux-amd64

LDFLAGS := -s -w \
	-X main.version=$(VERSION) \
	-X main.commit=$(COMMIT) \
	-X main.date=$(DATE)

.PHONY: help
help:
	@printf '%s\n' "Available targets:"
	@printf '  %-16s %s\n' "fmt" "Format Go code"
	@printf '  %-16s %s\n' "test" "Run Go tests"
	@printf '  %-16s %s\n' "vet" "Run go vet"
	@printf '  %-16s %s\n' "build" "Build linux/amd64 binary"
	@printf '  %-16s %s\n' "check-docs" "Validate doc scaffold"
	@printf '  %-16s %s\n' "check" "Run all checks"
	@printf '  %-16s %s\n' "package" "Create release tarball"
	@printf '  %-16s %s\n' "clean" "Remove build outputs"

.PHONY: fmt
fmt:
	gofmt -w ./cmd ./internal

.PHONY: test
test:
	go test ./...

.PHONY: vet
vet:
	go vet ./...

.PHONY: build
build:
	mkdir -p $(DIST_DIR)
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
		-trimpath \
		-ldflags "$(LDFLAGS)" \
		-o $(BIN) \
		./cmd/vps-watchdog

.PHONY: check-docs
check-docs:
	./scripts/check-docs.sh

.PHONY: check
check: fmt vet test check-docs build

.PHONY: package
package: build
	mkdir -p $(DIST_DIR)/package
	cp $(BIN) $(DIST_DIR)/package/$(APP)
	cp README.md LICENSE $(DIST_DIR)/package/
	cp -R packaging configs docs scripts AGENTS.md $(DIST_DIR)/package/
	tar -C $(DIST_DIR)/package -czf $(DIST_DIR)/$(APP)-linux-amd64.tar.gz .
	rm -rf $(DIST_DIR)/package

.PHONY: clean
clean:
	rm -rf $(DIST_DIR)
