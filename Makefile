VERSION ?= $(shell git describe --tags --always 2>/dev/null || echo dev)
LDFLAGS  = -s -w -X main.version=$(VERSION)
BIN      = fleet

.PHONY: build run test vet lint fmt check install uninstall dist clean

## build        — compile for the current platform
build:
	go build -trimpath -ldflags '$(LDFLAGS)' -o $(BIN) .

## run          — build and launch the TUI
run: build
	./$(BIN)

## test         — run all tests
test:
	go test ./...

## vet          — static analysis
vet:
	go vet ./...

## fmt          — format all Go files (check-only; exits non-zero on diff)
fmt:
	@gofmt -l . | tee /dev/stderr | (! grep -q .) || (echo "run: gofmt -w ." && exit 1)

## check        — vet + fmt + test (CI shortcut)
check: vet fmt test

## install      — build + copy to ~/.local/bin (or BIN_DIR)
install: build
	@mkdir -p "$${BIN_DIR:-$$HOME/.local/bin}"
	install -m 0755 $(BIN) "$${BIN_DIR:-$$HOME/.local/bin}/$(BIN)"
	@echo "installed → $${BIN_DIR:-$$HOME/.local/bin}/$(BIN)"

## uninstall    — remove the installed binary
uninstall:
	rm -f "$${BIN_DIR:-$$HOME/.local/bin}/$(BIN)"

## dist         — cross-compile all platforms via build.sh
dist:
	./build.sh $(VERSION)

## clean        — remove build artifacts
clean:
	rm -f $(BIN)
	rm -rf dist

## help         — show this list
help:
	@grep -E '^## ' $(MAKEFILE_LIST) | sed 's/^## /  /'
