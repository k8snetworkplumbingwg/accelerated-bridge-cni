# Package related
BINARY_NAME     = accelerated-bridge
PACKAGE         = accelerated-bridge-cni
ORG_PATH        = github.com/k8snetworkplumbingwg
REPO_PATH       = $(ORG_PATH)/$(PACKAGE)
BINDIR          = $(CURDIR)/bin
BUILDDIR        = $(CURDIR)/build
GOFILES         = $(shell find . -name *.go | grep -vE "(\/vendor\/)|(_test.go)")
PKGS            = $(or $(PKG),$(shell $(GO) list ./... | grep -v "^$(PACKAGE)/vendor/"))
TESTPKGS        = $(shell $(GO) list -f '{{ if or .TestGoFiles .XTestGoFiles }}{{ .ImportPath }}{{ end }}' $(PKGS))

# Version
VERSION?        = master
DATE            = `date -Iseconds`
COMMIT?         = `git rev-parse --verify HEAD`
LDFLAGS         = "-X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE)"

# Docker
IMAGE_BUILDER  ?= @docker
IMAGEDIR        = $(CURDIR)/images
DOCKERFILE      = $(CURDIR)/Dockerfile
TAG             = mellanox/accelerated-bridge-cni
# Accept proxy settings for docker
# To pass proxy for Docker invoke it as 'make image HTTP_POXY=http://192.168.0.1:8080'
DOCKERARGS      =
ifdef HTTP_PROXY
	DOCKERARGS += --build-arg http_proxy=$(HTTP_PROXY)
endif
ifdef HTTPS_PROXY
	DOCKERARGS += --build-arg https_proxy=$(HTTPS_PROXY)
endif

# Go tools
GO              = go
GOLANGCI_LINT   = $(BINDIR)/golangci-lint
# golangci-lint version should be updated periodically
# we keep it fixed to avoid it from unexpectedly failing on the project
# in case of a version bump
GOLANGCI_LINT_VER = v1.46.2
TIMEOUT         = 15
V               = 0
Q               = $(if $(filter 1,$V),,@)

.PHONY: all
all: lint build test-coverage

$(BINDIR):
	@mkdir -p $@

$(BUILDDIR): | ; $(info Creating build directory...)
	@mkdir -p $@

build: $(BUILDDIR)/$(BINARY_NAME) ; $(info Building $(BINARY_NAME)...) ## Build executable file
	$(info Done!)

$(BUILDDIR)/$(BINARY_NAME): $(GOFILES) | $(BUILDDIR)
	@cd cmd/$(BINARY_NAME) && CGO_ENABLED=0 $(GO) build -o $(BUILDDIR)/$(BINARY_NAME) -tags no_openssl -ldflags $(LDFLAGS) -v

# Tools
$(GOLANGCI_LINT): | $(BASE) ; $(info  installing golangci-lint...)
	$(call go-install-tool,$(GOLANGCI_LINT),github.com/golangci/golangci-lint/cmd/golangci-lint@$(GOLANGCI_LINT_VER))

GOVERALLS = $(BINDIR)/goveralls
$(GOVERALLS): | $(BASE) ; $(info  installing goveralls...)
	$(call go-install-tool,$(GOVERALLS),github.com/mattn/goveralls@latest)

# Tests
TEST_TARGETS := test-default test-bench test-short test-verbose test-race
.PHONY: $(TEST_TARGETS) test-xml check test tests
test-bench:   ARGS=-run=__absolutelynothing__ -bench=. ## Run benchmarks
test-short:   ARGS=-short        ## Run only short tests
test-verbose: ARGS=-v            ## Run tests in verbose mode with coverage reporting
test-race:    ARGS=-race         ## Run tests with race detector
$(TEST_TARGETS): NAME=$(MAKECMDGOALS:test-%=%)
$(TEST_TARGETS): test
check test tests: lint | $(BASE) ; $(info  running $(NAME:%=% )tests...) @ ## Run tests
	$Q $(GO) test -timeout $(TIMEOUT)s $(ARGS) $(TESTPKGS)

test-xml: lint | $(BASE) $(GO2XUNIT) ; $(info  running $(NAME:%=% )tests...) @ ## Run tests with xUnit output
	$Q 2>&1 $(GO) test -timeout 20s -v $(TESTPKGS) | tee test/tests.output
	$(GO2XUNIT) -fail -input test/tests.output -output test/tests.xml

COVERAGE_MODE = count
COVER_PROFILE = accelerated-bridge.cover
test-coverage-tools: | $(GOVERALLS)
test-coverage: test-coverage-tools | $(BASE) ; $(info  running coverage tests...) @ ## Run coverage tests
	$Q $(GO) test -covermode=$(COVERAGE_MODE) -coverprofile=$(COVER_PROFILE) ./...

.PHONY: upload-coverage
upload-coverage: test-coverage-tools | $(BASE) ; $(info  uploading coverage results...) @ ## Upload coverage report
	$(GOVERALLS) -coverprofile=$(COVER_PROFILE) -service=github

.PHONY: lint
lint: | $(BASE) $(GOLANGCI_LINT) ; $(info  running golangci-lint...) @ ## Run golangci-lint
	$Q $(GOLANGCI_LINT) run --timeout=10m

# Docker image
.PHONY: image
image: | $(BASE) ; $(info Building Docker image...)
	$(IMAGE_BUILDER) build -t $(TAG) -f $(DOCKERFILE)  $(CURDIR) $(DOCKERARGS)

# Dependency management
.PHONY: deps-update
deps-update: ; $(info  updating dependencies...)
	@$(GO) mod tidy

# Misc
.PHONY: clean
clean: ; $(info  Cleaning...)	@ ## Cleanup everything
	@rm -rf $(BUILDDIR)
	@rm -rf test
	@rm -rf $(BINDIR)

.PHONY: help
help: ## Show this message
	@grep -E '^[ a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

# go-install-tool will 'go install' any package $2 and install it to $1.
define go-install-tool
@[ -f $(1) ] || { \
echo "Downloading $(2)" ;\
GOBIN=$(BINDIR) go install $(2) ;\
}
endef

