# Package related
BINARY_NAME     = accelerated-bridge
PACKAGE         = accelerated-bridge-cni
ORG_PATH        = github.com/k8snetworkplumbingwg
REPO_PATH       = $(ORG_PATH)/$(PACKAGE)
GOPATH          = $(CURDIR)/.gopath
GOBIN           = $(CURDIR)/bin
BUILDDIR        = $(CURDIR)/build
BASE            = $(GOPATH)/src/$(REPO_PATH)
GOFILES         = $(shell find . -name *.go | grep -vE "(\/vendor\/)|(_test.go)")
PKGS            = $(or $(PKG),$(shell cd $(BASE) && env GOPATH=$(GOPATH) $(GO) list ./... | grep -v "^$(PACKAGE)/vendor/"))
TESTPKGS        = $(shell env GOPATH=$(GOPATH) $(GO) list -f '{{ if or .TestGoFiles .XTestGoFiles }}{{ .ImportPath }}{{ end }}' $(PKGS))

export GOPATH
export GOBIN

# Version
VERSION?        = master
DATE            = `date -Iseconds`
COMMIT?         = `git rev-parse --verify HEAD`
LDFLAGS         = "-X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE)"

# Docker
IMAGE_BUILDER  ?= @docker
IMAGEDIR        = $(BASE)/images
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
GOLANGCI_LINT   = $(GOBIN)/golangci-lint
# golangci-lint version should be updated periodically
# we keep it fixed to avoid it from unexpectedly failing on the project
# in case of a version bump
GOLANGCI_LINT_VER = v1.23.8
TIMEOUT         = 15
V               = 0
Q               = $(if $(filter 1,$V),,@)

.PHONY: all
all: lint build test-coverage

$(BASE): ; $(info  setting GOPATH...)
	@mkdir -p $(dir $@)
	@ln -sf $(CURDIR) $@

$(GOBIN):
	@mkdir -p $@

$(BUILDDIR): | $(BASE) ; $(info Creating build directory...)
	@cd $(BASE) && mkdir -p $@

build: $(BUILDDIR)/$(BINARY_NAME) ; $(info Building $(BINARY_NAME)...) ## Build executable file
	$(info Done!)

$(BUILDDIR)/$(BINARY_NAME): $(GOFILES) | $(BUILDDIR)
	@cd $(BASE)/cmd/$(BINARY_NAME) && CGO_ENABLED=0 $(GO) build -o $(BUILDDIR)/$(BINARY_NAME) -tags no_openssl -ldflags $(LDFLAGS) -v


# Tools
$(GOLANGCI_LINT): | $(BASE) ; $(info  building golangci-lint...)
	$Q curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(GOBIN) $(GOLANGCI_LINT_VER)

GOVERALLS = $(GOBIN)/goveralls
$(GOBIN)/goveralls: | $(BASE) ; $(info  building goveralls...)
	$Q env GO111MODULE=off go get github.com/mattn/goveralls

MOCKERY = $(shell pwd)/bin/mockery
mockery: ## Download mockery if necessary.
	$(call go-get-tool,$(MOCKERY),github.com/vektra/mockery/v2@v2.8.0)

# go-get-tool will 'go get' any package $2 and install it to $1.
PROJECT_DIR := $(shell dirname $(abspath $(lastword $(MAKEFILE_LIST))))
define go-get-tool
@[ -f $(1) ] || { \
set -e ;\
TMP_DIR=$$(mktemp -d) ;\
cd $$TMP_DIR ;\
go mod init tmp ;\
echo "Downloading $(2)" ;\
GOBIN=$(PROJECT_DIR)/bin go get $(2) ;\
rm -rf $$TMP_DIR ;\
}
endef

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
	$Q cd $(BASE) && $(GO) test -timeout $(TIMEOUT)s $(ARGS) $(TESTPKGS)

test-xml: lint | $(BASE) $(GO2XUNIT) ; $(info  running $(NAME:%=% )tests...) @ ## Run tests with xUnit output
	$Q cd $(BASE) && 2>&1 $(GO) test -timeout 20s -v $(TESTPKGS) | tee test/tests.output
	$(GO2XUNIT) -fail -input test/tests.output -output test/tests.xml

COVERAGE_MODE = count
COVER_PROFILE = accelerated-bridge.cover
test-coverage-tools: | $(GOVERALLS)
test-coverage: test-coverage-tools | $(BASE) ; $(info  running coverage tests...) @ ## Run coverage tests
	$Q cd $(BASE); $(GO) test -covermode=$(COVERAGE_MODE) -coverprofile=$(COVER_PROFILE) ./...

.PHONY: upload-coverage
upload-coverage: test-coverage-tools | $(BASE) ; $(info  uploading coverage results...) @ ## Upload coverage report
	$(GOVERALLS) -coverprofile=$(COVER_PROFILE) -service=github

.PHONY: lint
lint: | $(BASE) $(GOLANGCI_LINT) ; $(info  running golangci-lint...) @ ## Run golangci-lint
	$Q mkdir -p $(BASE)/test
	$Q cd $(BASE) && ret=0 && \
		test -z "$$($(GOLANGCI_LINT) run | tee $(BASE)/test/lint.out)" || ret=1 ; \
		cat $(BASE)/test/lint.out ; rm -rf $(BASE)/test ; \
	 exit $$ret

# Docker image
.PHONY: image
image: | $(BASE) ; $(info Building Docker image...)
	$(IMAGE_BUILDER) build -t $(TAG) -f $(DOCKERFILE)  $(CURDIR) $(DOCKERARGS)

# Dependency management
.PHONY: deps-update
deps-update: ; $(info  updating dependencies...)
	@$(GO) mod tidy && $(GO) mod vendor

# Misc
.PHONY: clean
clean: ; $(info  Cleaning...)	@ ## Cleanup everything
	@$(GO) clean -modcache
	@rm -rf $(GOPATH)
	@rm -rf $(BUILDDIR)
	@rm -rf test
	@rm -rf bin

.PHONY: help
help: ## Show this message
	@grep -E '^[ a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'
