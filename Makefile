BUILDS_DIR := builds
RELEASE ?= $(shell git describe --tags --always --dirty)

PLATFORMS := \
	linux-amd64 \
	linux-arm64 \
	darwin-amd64 \
	darwin-arm64 \
	freebsd-amd64 \
	freebsd-arm64 \
	windows-amd64 \
	windows-arm64

COMMANDS := cgrep cindex csearch csweb

.PHONY: all
all: test build

.PHONY: test
test:
	go test ./...

.PHONY: race
race:
	go test -race ./...

.PHONY: build
build:
	go build ./cmd/...

.PHONY: install
install:
	go install ./cmd/...

.PHONY: release
release:
	@for platform in $(PLATFORMS); do \
		GOOS=$${platform%-*}; \
		GOARCH=$${platform#*-}; \
		outdir="$(BUILDS_DIR)/$(RELEASE)/$${GOOS}_$${GOARCH}"; \
		mkdir -p "$$outdir"; \
		for cmd in $(COMMANDS); do \
			exe="$$cmd"; \
			if [ "$$GOOS" = "windows" ]; then exe="$$cmd.exe"; fi; \
			echo "building $$cmd for $$GOOS/$$GOARCH"; \
			GOOS=$$GOOS GOARCH=$$GOARCH go build -o "$$outdir/$$exe" "./cmd/$$cmd"; \
		done; \
		cp LICENSE "$$outdir/"; \
	done

.PHONY: clean
clean:
	rm -rf "$(BUILDS_DIR)"
