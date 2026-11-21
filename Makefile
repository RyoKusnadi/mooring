# Mooring build and checks. `make check` must pass before any phase is done.

GO       ?= go
PNPM     ?= pnpm
GOLANGCI ?= $(shell $(GO) env GOPATH)/bin/golangci-lint
GO_DIRS  := cmd internal
VERSION  ?= $(shell git describe --tags --always --dirty)
# The same flags build and release use, so e2e runs the kind of binary that ships.
GOFLAGS_BUILD := -trimpath -ldflags "-s -w -X main.version=$(VERSION)"

.PHONY: build build-e2e release cross run dev check check-go check-web check-size e2e screenshots fmt clean

build: web/node_modules
	cd web && $(PNPM) run -s build
	@touch internal/webui/dist/.gitkeep
	CGO_ENABLED=0 $(GO) build $(GOFLAGS_BUILD) -o bin/mooring ./cmd/mooring

# The binary browser tests run: the release build plus the test-only flags behind the e2e tag.
build-e2e: build
	CGO_ENABLED=0 $(GO) build -tags e2e $(GOFLAGS_BUILD) -o bin/mooring-e2e ./cmd/mooring

# Tarballs and SHA256SUMS for four targets in bin/release/<version>/. Refuses a dirty tree unless ALLOW_DIRTY=1.
release: web/node_modules
	@case "$(VERSION)" in *-dirty) [ "$(ALLOW_DIRTY)" = 1 ] || { echo "release: $(VERSION) has uncommitted changes. Commit them, or run with ALLOW_DIRTY=1."; exit 1; } ;; esac
	cd web && $(PNPM) run -s build
	@touch internal/webui/dist/.gitkeep
	VERSION=$(VERSION) ALLOW_DIRTY=$(ALLOW_DIRTY) scripts/release.sh

# Catches code that only builds on this machine: Linux vet, and an Intel macOS build.
cross:
	GOOS=linux $(GO) vet ./...
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 $(GO) build -o /dev/null ./cmd/mooring

run: build
	./bin/mooring

# Hot-reloading web app on :5173; run `make run` in another terminal for the API.
dev: web/node_modules
	cd web && $(PNPM) run dev

web/node_modules: web/package.json web/pnpm-lock.yaml
	cd web && $(PNPM) install --frozen-lockfile
	@touch $@

check: check-go cross check-web build check-size e2e
	@echo "make check: all passed"

check-go:
	@out=$$(gofmt -l $(GO_DIRS)); if [ -n "$$out" ]; then echo "gofmt needed (run make fmt):"; echo "$$out"; exit 1; fi
	$(GO) vet ./...
	$(GO) vet -tags e2e ./cmd/...
	@command -v $(GOLANGCI) >/dev/null || { echo "golangci-lint not found at $(GOLANGCI)"; exit 1; }
	$(GOLANGCI) run ./...
	$(GO) test -race -count=1 ./...

check-web: web/node_modules
	cd web && $(PNPM) run -s check
	cd web && $(PNPM) run -s test

check-size:
	node scripts/check-size.mjs .

e2e: build-e2e
	rm -rf web/.e2e-data
	cd web && $(PNPM) run -s e2e

# Retakes the pictures in docs/guide/images with demo content. Not part of check. Port base 7450.
screenshots: build-e2e
	rm -rf web/.docs-data
	cd web && MOORING_E2E_PORT=7450 node node_modules/@playwright/test/cli.js test -c playwright.docs.config.ts
	@command -v ffmpeg >/dev/null || { echo "make screenshots needs ffmpeg to make the GIF (demo-agent-chat.gif)"; exit 1; }
	ffmpeg -loglevel error -y -f concat -i web/.docs-data/frames/list.txt -vf "fps=12,scale=960:-1:flags=lanczos,split[a][b];[a]palettegen=max_colors=96[p];[b][p]paletteuse=dither=bayer:bayer_scale=4" docs/guide/images/demo-agent-chat.gif
	rm -rf web/.docs-data

fmt:
	gofmt -w $(GO_DIRS)

clean:
	rm -rf bin web/test-results web/playwright-report web/.e2e-data web/.docs-data
	find internal/webui/dist -mindepth 1 ! -name .gitkeep -delete
