ROOT_DIR     := $(shell dirname $(realpath $(lastword $(MAKEFILE_LIST))))
IMAGE_NAME   := mtg
APP_NAME     := $(IMAGE_NAME)

GOLANGCI_LINT_VERSION := v1.54.2

VERSION            := $(shell git describe --exact-match HEAD 2>/dev/null || git describe --tags --always)
COMMON_BUILD_FLAGS := -trimpath -mod=readonly -ldflags="-extldflags '-static' -s -w -X 'main.version=$(VERSION)'"

# PGO configuration
PGO_PROFILE := default.pgo
PGO_BUILD_FLAGS := $(COMMON_BUILD_FLAGS)
ifneq (,$(wildcard $(PGO_PROFILE)))
	PGO_BUILD_FLAGS += -pgo=$(PGO_PROFILE)
endif

FUZZ_FLAGS := -fuzztime=120s

GOBIN  := $(ROOT_DIR)/.bin
GOTOOL := env "GOBIN=$(GOBIN)" "PATH=$(GOBIN):$(PATH)"

# Go 1.25 optimization flags
GOFLAGS := -buildvcs=auto

# -----------------------------------------------------------------------------

.PHONY: all
all: build

.PHONY: build
build:
	@go build $(PGO_BUILD_FLAGS) -o "$(APP_NAME)"

$(APP_NAME): build

.PHONY: static
static:
	@CGO_ENABLED=0 GOOS=linux go build \
		$(PGO_BUILD_FLAGS) \
		-tags netgo \
		-a \
		-o "$(APP_NAME)"

.PHONY: build-nopgo
build-nopgo:
	@go build $(COMMON_BUILD_FLAGS) -o "$(APP_NAME)"

.PHONY: pgo-generate
pgo-generate: build-nopgo
	@echo "Starting CPU profiling..."
	@echo "Run your application with typical workload, then press Ctrl+C"
	@echo "Example: ./$(APP_NAME) [your-args] &"
	@echo "         kill -INT \$$!"
	@echo ""
	@echo "To generate profile manually, run:"
	@echo "  ./$(APP_NAME) -cpuprofile=cpu.prof [your-args]"
	@echo ""
	@echo "Then run: make pgo-merge"

.PHONY: pgo-merge
pgo-merge:
	@if [ ! -f cpu.prof ]; then \
		echo "Error: cpu.prof not found. Run your application with -cpuprofile=cpu.prof first"; \
		exit 1; \
	fi
	@echo "Converting CPU profile to PGO format..."
	@go tool pprof -proto cpu.prof > $(PGO_PROFILE)
	@echo "PGO profile created: $(PGO_PROFILE)"
	@echo "Build with PGO: make build"

.PHONY: pgo-clean
pgo-clean:
	@rm -f $(PGO_PROFILE) cpu.prof
	@echo "PGO profiles cleaned"

.PHONY: pgo-info
pgo-info:
	@if [ -f $(PGO_PROFILE) ]; then \
		echo "PGO profile exists: $(PGO_PROFILE)"; \
		ls -lh $(PGO_PROFILE); \
		echo "PGO is ENABLED for builds"; \
	else \
		echo "No PGO profile found"; \
		echo "PGO is DISABLED for builds"; \
		echo ""; \
		echo "To enable PGO:"; \
		echo "  1. Build without PGO: make build-nopgo"; \
		echo "  2. Run with profiling: ./$(APP_NAME) -cpuprofile=cpu.prof [args]"; \
		echo "  3. Generate PGO profile: make pgo-merge"; \
		echo "  4. Rebuild with PGO: make build"; \
	fi

.PHONY: vendor
vendor: go.mod go.sum
	@go mod vendor

.bin:
	@mkdir -p "$(GOBIN)"

.PHONY: fmt
fmt: .bin
	@$(GOTOOL) gofumpt -w -extra "$(ROOT_DIR)"

.PHONY: test
test:
	@go test -shuffle=on -v ./...

.PHONY: citest
citest:
	@go test -coverprofile=coverage.txt -covermode=atomic -parallel 2 -race -shuffle=on -v ./...

.PHONY: clean
clean:
	@git clean -xfd && \
		git reset --hard >/dev/null && \
		git submodule foreach --recursive sh -c 'git clean -xfd && git reset --hard' >/dev/null

.PHONY: lint
lint: .bin
	@$(GOTOOL) golangci-lint run

.PHONY: release
release: .bin
	@$(GOTOOL) goreleaser release --snapshot --clean && \
		find "$(ROOT_DIR)/dist" -mindepth 1 -maxdepth 1 -type d -exec rm -rf {} + && \
		rm -f "$(ROOT_DIR)/dist/config.yaml"

.PHONY: docker
docker:
	@docker build --pull -t "$(IMAGE_NAME)" "$(ROOT_DIR)"

.PHONY: doc
doc: .bin
	@$(GOTOOL) godoc -http 0.0.0.0:10000

.PHONY: install-tools
install-tools: install-tools-lint install-tools-godoc install-tools-gofumpt install-tools-goreleaser

.PHONY: install-tools-lint
install-tools-lint: .bin
	@curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh \
		| sh -s -- -b "$(GOBIN)" "$(GOLANGCI_LINT_VERSION)"

.PHONY: install-tools-godoc
install-tools-godoc: .bin
	@$(GOTOOL) go install golang.org/x/tools/cmd/godoc@latest

.PHONY: install-tools-gofumpt
install-tools-gofumpt: .bin
	@$(GOTOOL) go install mvdan.cc/gofumpt@latest

.PHONY: install-tools-goreleaser
install-tools-goreleaser: .bin
	@$(GOTOOL) go install github.com/goreleaser/goreleaser@latest

.PHONY: update-deps
update-deps:
	@go get -u && go mod tidy -go=1.25

.PHONY: fuzz
fuzz: fuzz-ClientHello fuzz-ServerGenerateHandshakeFrame fuzz-ClientHandshake fuzz-ServerReceive fuzz-ServerSend

.PHONY: fuzz-ClientHello
fuzz-ClientHello:
	@go test -fuzz=FuzzClientHello $(FUZZ_FLAGS) ./mtglib/internal/faketls

.PHONY: fuzz-ServerGenerateHandshakeFrame
fuzz-ServerGenerateHandshakeFrame:
	@go test -fuzz=FuzzServerGenerateHandshakeFrame $(FUZZ_FLAGS) ./mtglib/internal/obfuscated2

.PHONY: fuzz-ClientHandshake
fuzz-ClientHandshake:
	@go test -fuzz=FuzzClientHandshake $(FUZZ_FLAGS) ./mtglib/internal/obfuscated2

.PHONY: fuzz-ServerReceive
fuzz-ServerReceive:
	@go test -fuzz=FuzzServerReceive $(FUZZ_FLAGS) ./mtglib/internal/obfuscated2

.PHONY: fuzz-ServerSend
fuzz-ServerSend:
	@go test -fuzz=FuzzServerSend $(FUZZ_FLAGS) ./mtglib/internal/obfuscated2