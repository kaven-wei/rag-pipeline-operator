# Image URL to use all building/pushing image targets
IMG ?= controller:latest

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

# CONTAINER_TOOL defines the container tool to be used for building images.
# Be aware that the target commands are only tested with Docker which is
# scaffolded by default. However, you might want to replace it to use other
# tools. (i.e. podman)
CONTAINER_TOOL ?= docker

# Setting SHELL to bash allows bash commands to be executed by recipes.
# Options are set to exit when a recipe line exits non-zero or a piped command fails.
SHELL = /usr/bin/env bash -o pipefail
.SHELLFLAGS = -ec

# Colors for terminal output
CYAN := \033[36m
GREEN := \033[32m
YELLOW := \033[33m
RED := \033[31m
RESET := \033[0m

.PHONY: all
all: build

##@ General

# The help target prints out all targets with their descriptions organized
# beneath their categories. The categories are represented by '##@' and the
# target descriptions by '##'. The awk command is responsible for reading the
# entire set of makefiles included in this invocation, looking for lines of the
# file as xyz: ## something, and then pretty-format the target and help. Then,
# if there's a line with ##@ something, that gets pretty-printed as a category.
# More info on the usage of ANSI control characters for terminal formatting:
# https://en.wikipedia.org/wiki/ANSI_escape_code#SGR_parameters
# More info on the awk command:
# http://linuxcommand.org/lc3_adv_awk.php

.PHONY: help
help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

##@ Development

.PHONY: tidy
tidy: ## Run go mod tidy to clean up dependencies.
	@echo -e "$(CYAN)Running go mod tidy...$(RESET)"
	go mod tidy
	@echo -e "$(GREEN)✓ Dependencies tidied$(RESET)"

.PHONY: verify-deps
verify-deps: ## Verify that dependencies are correct.
	@echo -e "$(CYAN)Verifying dependencies...$(RESET)"
	go mod verify
	@echo -e "$(GREEN)✓ Dependencies verified$(RESET)"

.PHONY: download
download: ## Download all dependencies.
	@echo -e "$(CYAN)Downloading dependencies...$(RESET)"
	go mod download
	@echo -e "$(GREEN)✓ Dependencies downloaded$(RESET)"

.PHONY: deps
deps: tidy download verify-deps ## Full dependency management: tidy, download, and verify.

.PHONY: manifests
manifests: controller-gen ## Generate WebhookConfiguration, ClusterRole and CustomResourceDefinition objects.
	@echo -e "$(CYAN)Generating manifests...$(RESET)"
	@mkdir -p config/crd/bases
	"$(CONTROLLER_GEN)" rbac:roleName=manager-role crd webhook paths="./..." output:crd:artifacts:config=config/crd/bases
	@echo -e "$(GREEN)✓ Manifests generated$(RESET)"

.PHONY: generate
generate: controller-gen ## Generate code containing DeepCopy, DeepCopyInto, and DeepCopyObject method implementations.
	@echo -e "$(CYAN)Generating code...$(RESET)"
	"$(CONTROLLER_GEN)" object:headerFile="hack/boilerplate.go.txt" paths="./..."
	@echo -e "$(GREEN)✓ Code generated$(RESET)"

.PHONY: fmt
fmt: ## Run go fmt against code.
	@echo -e "$(CYAN)Running go fmt...$(RESET)"
	go fmt ./...
	@echo -e "$(GREEN)✓ Code formatted$(RESET)"

.PHONY: vet
vet: ## Run go vet against code.
	@echo -e "$(CYAN)Running go vet...$(RESET)"
	go vet ./...
	@echo -e "$(GREEN)✓ Code vetted$(RESET)"

.PHONY: test
test: manifests generate fmt vet setup-envtest ## Run tests.
	KUBEBUILDER_ASSETS="$(shell "$(ENVTEST)" use $(ENVTEST_K8S_VERSION) --bin-dir "$(LOCALBIN)" -p path)" go test $$(go list ./... | grep -v /e2e) -coverprofile cover.out

# TODO(user): To use a different vendor for e2e tests, modify the setup under 'tests/e2e'.
# The default setup assumes Kind is pre-installed and builds/loads the Manager Docker image locally.
# CertManager is installed by default; skip with:
# - CERT_MANAGER_INSTALL_SKIP=true
KIND_CLUSTER ?= rag-operator-test-e2e

.PHONY: setup-test-e2e
setup-test-e2e: ## Set up a Kind cluster for e2e tests if it does not exist
	@command -v $(KIND) >/dev/null 2>&1 || { \
		echo "Kind is not installed. Please install Kind manually."; \
		exit 1; \
	}
	@case "$$($(KIND) get clusters)" in \
		*"$(KIND_CLUSTER)"*) \
			echo "Kind cluster '$(KIND_CLUSTER)' already exists. Skipping creation." ;; \
		*) \
			echo "Creating Kind cluster '$(KIND_CLUSTER)'..."; \
			$(KIND) create cluster --name $(KIND_CLUSTER) ;; \
	esac

.PHONY: test-e2e
test-e2e: setup-test-e2e manifests generate fmt vet ## Run the e2e tests. Expected an isolated environment using Kind.
	KIND=$(KIND) KIND_CLUSTER=$(KIND_CLUSTER) go test -tags=e2e ./test/e2e/ -v -ginkgo.v
	$(MAKE) cleanup-test-e2e

.PHONY: cleanup-test-e2e
cleanup-test-e2e: ## Tear down the Kind cluster used for e2e tests
	@$(KIND) delete cluster --name $(KIND_CLUSTER)

.PHONY: lint
lint: golangci-lint ## Run golangci-lint linter
	"$(GOLANGCI_LINT)" run

.PHONY: lint-fix
lint-fix: golangci-lint ## Run golangci-lint linter and perform fixes
	"$(GOLANGCI_LINT)" run --fix

.PHONY: lint-config
lint-config: golangci-lint ## Verify golangci-lint linter configuration
	"$(GOLANGCI_LINT)" config verify

##@ Build

.PHONY: build
build: manifests generate fmt vet ## Build manager binary.
	@echo -e "$(CYAN)Building manager binary...$(RESET)"
	go build -o bin/manager cmd/main.go
	@echo -e "$(GREEN)✓ Build complete: bin/manager$(RESET)"

.PHONY: build-fast
build-fast: ## Quick build without code generation (for development iteration).
	@echo -e "$(CYAN)Quick building manager binary...$(RESET)"
	go build -o bin/manager cmd/main.go
	@echo -e "$(GREEN)✓ Quick build complete: bin/manager$(RESET)"

.PHONY: run
run: manifests generate fmt vet ## Run a controller from your host.
	go run ./cmd/main.go

.PHONY: run-fast
run-fast: ## Quick run without code generation (for development iteration).
	go run ./cmd/main.go

# If you wish to build the manager image targeting other platforms you can use the --platform flag.
# (i.e. docker build --platform linux/arm64). However, you must enable docker buildKit for it.
# More info: https://docs.docker.com/develop/develop-images/build_enhancements/
.PHONY: docker-build
docker-build: ## Build docker image with the manager.
	$(CONTAINER_TOOL) build -t ${IMG} .

.PHONY: docker-push
docker-push: ## Push docker image with the manager.
	$(CONTAINER_TOOL) push ${IMG}

# PLATFORMS defines the target platforms for the manager image be built to provide support to multiple
# architectures. (i.e. make docker-buildx IMG=myregistry/mypoperator:0.0.1). To use this option you need to:
# - be able to use docker buildx. More info: https://docs.docker.com/build/buildx/
# - have enabled BuildKit. More info: https://docs.docker.com/develop/develop-images/build_enhancements/
# - be able to push the image to your registry (i.e. if you do not set a valid value via IMG=<myregistry/image:<tag>> then the export will fail)
# To adequately provide solutions that are compatible with multiple platforms, you should consider using this option.
PLATFORMS ?= linux/arm64,linux/amd64,linux/s390x,linux/ppc64le
.PHONY: docker-buildx
docker-buildx: ## Build and push docker image for the manager for cross-platform support
	# copy existing Dockerfile and insert --platform=${BUILDPLATFORM} into Dockerfile.cross, and preserve the original Dockerfile
	sed -e '1 s/\(^FROM\)/FROM --platform=\$$\{BUILDPLATFORM\}/; t' -e ' 1,// s//FROM --platform=\$$\{BUILDPLATFORM\}/' Dockerfile > Dockerfile.cross
	- $(CONTAINER_TOOL) buildx create --name rag-operator-builder
	$(CONTAINER_TOOL) buildx use rag-operator-builder
	- $(CONTAINER_TOOL) buildx build --push --platform=$(PLATFORMS) --tag ${IMG} -f Dockerfile.cross .
	- $(CONTAINER_TOOL) buildx rm rag-operator-builder
	rm Dockerfile.cross

.PHONY: build-installer
build-installer: manifests generate kustomize ## Generate a consolidated YAML with CRDs and deployment.
	mkdir -p dist
	cd config/manager && "$(KUSTOMIZE)" edit set image controller=${IMG}
	"$(KUSTOMIZE)" build config/default > dist/install.yaml

##@ Deployment

ifndef ignore-not-found
  ignore-not-found = false
endif

.PHONY: install
install: manifests kustomize ## Install CRDs into the K8s cluster specified in ~/.kube/config.
	@echo -e "$(CYAN)Installing CRDs...$(RESET)"
	@out="$$( "$(KUSTOMIZE)" build config/crd 2>/dev/null || true )"; \
	if [ -n "$$out" ]; then echo "$$out" | "$(KUBECTL)" apply -f -; else echo "No CRDs to install; skipping."; fi
	@echo -e "$(GREEN)✓ CRDs installed$(RESET)"

.PHONY: uninstall
uninstall: manifests kustomize ## Uninstall CRDs from the K8s cluster specified in ~/.kube/config. Call with ignore-not-found=true to ignore resource not found errors during deletion.
	@out="$$( "$(KUSTOMIZE)" build config/crd 2>/dev/null || true )"; \
	if [ -n "$$out" ]; then echo "$$out" | "$(KUBECTL)" delete --ignore-not-found=$(ignore-not-found) -f -; else echo "No CRDs to delete; skipping."; fi

.PHONY: deploy
deploy: manifests kustomize ## Deploy controller to the K8s cluster specified in ~/.kube/config.
	cd config/manager && "$(KUSTOMIZE)" edit set image controller=${IMG}
	"$(KUSTOMIZE)" build config/default | "$(KUBECTL)" apply -f -

.PHONY: undeploy
undeploy: kustomize ## Undeploy controller from the K8s cluster specified in ~/.kube/config. Call with ignore-not-found=true to ignore resource not found errors during deletion.
	"$(KUSTOMIZE)" build config/default | "$(KUBECTL)" delete --ignore-not-found=$(ignore-not-found) -f -

##@ Cleanup & Cache

.PHONY: clean
clean: ## Remove build artifacts and generated files.
	@echo -e "$(CYAN)Cleaning build artifacts...$(RESET)"
	rm -rf bin/
	rm -rf cover.out
	rm -rf dist/
	rm -f Dockerfile.cross
	@echo -e "$(GREEN)✓ Build artifacts cleaned$(RESET)"

.PHONY: clean-tools
clean-tools: ## Remove downloaded tool binaries.
	@echo -e "$(CYAN)Cleaning tool binaries...$(RESET)"
	rm -rf $(LOCALBIN)
	@echo -e "$(GREEN)✓ Tool binaries cleaned$(RESET)"

.PHONY: clean-generated
clean-generated: ## Remove generated code (DeepCopy, CRDs, etc.).
	@echo -e "$(YELLOW)Warning: This will remove generated files!$(RESET)"
	rm -f api/v1alpha1/zz_generated.deepcopy.go
	rm -rf config/crd/bases/
	@echo -e "$(GREEN)✓ Generated files cleaned$(RESET)"

.PHONY: clean-all
clean-all: clean clean-tools clean-generated ## Remove all generated files, tools, and build artifacts.
	@echo -e "$(GREEN)✓ All cleaned$(RESET)"

.PHONY: clean-cache
clean-cache: ## Clean Go module cache (use with caution).
	@echo -e "$(YELLOW)Cleaning Go module cache...$(RESET)"
	go clean -modcache
	@echo -e "$(GREEN)✓ Module cache cleaned$(RESET)"

##@ Development Utilities

.PHONY: dev-setup
dev-setup: deps controller-gen kustomize ## Set up development environment (download deps and tools).
	@echo -e "$(GREEN)✓ Development environment ready$(RESET)"

.PHONY: dev-refresh
dev-refresh: tidy generate manifests fmt vet ## Refresh all generated code and manifests.
	@echo -e "$(GREEN)✓ All code and manifests refreshed$(RESET)"

.PHONY: pre-commit
pre-commit: tidy fmt vet lint ## Run all checks before committing.
	@echo -e "$(GREEN)✓ Pre-commit checks passed$(RESET)"

.PHONY: check
check: verify-deps vet lint test ## Run all verification and tests.
	@echo -e "$(GREEN)✓ All checks passed$(RESET)"

.PHONY: quick-test
quick-test: ## Run tests without regenerating code (faster for TDD).
	go test $$(go list ./... | grep -v /e2e) -v

.PHONY: show-coverage
show-coverage: test ## Run tests and display coverage report.
	go tool cover -html=cover.out

.PHONY: watch
watch: ## Watch for file changes and rebuild (requires entr).
	@command -v entr >/dev/null 2>&1 || { echo "entr is required. Install with: brew install entr (macOS) or apt install entr (Linux)"; exit 1; }
	@echo -e "$(CYAN)Watching for changes...$(RESET)"
	find . -name '*.go' | entr -r make build-fast

.PHONY: apply-samples
apply-samples: ## Apply sample CRs to the cluster.
	@echo -e "$(CYAN)Applying sample CRs...$(RESET)"
	"$(KUBECTL)" apply -f config/samples/
	@echo -e "$(GREEN)✓ Samples applied$(RESET)"

.PHONY: delete-samples
delete-samples: ## Delete sample CRs from the cluster.
	@echo -e "$(CYAN)Deleting sample CRs...$(RESET)"
	"$(KUBECTL)" delete -f config/samples/ --ignore-not-found=true
	@echo -e "$(GREEN)✓ Samples deleted$(RESET)"

.PHONY: logs
logs: ## Tail controller manager logs.
	"$(KUBECTL)" logs -f -n rag-operator-system deployment/rag-operator-controller-manager -c manager

.PHONY: describe
describe: ## Describe all RAG CRDs in the cluster.
	@echo -e "$(CYAN)=== DocumentSets ===$(RESET)"
	"$(KUBECTL)" get documentsets -A
	@echo -e "$(CYAN)=== EmbeddingJobs ===$(RESET)"
	"$(KUBECTL)" get embeddingjobs -A
	@echo -e "$(CYAN)=== IndexJobs ===$(RESET)"
	"$(KUBECTL)" get indexjobs -A

.PHONY: version
version: ## Display version information.
	@echo -e "$(CYAN)RAG Pipeline Operator$(RESET)"
	@echo "Go version: $$(go version | cut -d' ' -f3)"
	@echo "Controller-gen: $(CONTROLLER_TOOLS_VERSION)"
	@echo "Kustomize: $(KUSTOMIZE_VERSION)"
	@echo "Image: $(IMG)"

##@ Dependencies

## Location to install dependencies to
LOCALBIN ?= $(shell pwd)/bin
$(LOCALBIN):
	mkdir -p "$(LOCALBIN)"

## Tool Binaries
KUBECTL ?= kubectl
KIND ?= kind
KUSTOMIZE ?= $(LOCALBIN)/kustomize
CONTROLLER_GEN ?= $(LOCALBIN)/controller-gen
ENVTEST ?= $(LOCALBIN)/setup-envtest
GOLANGCI_LINT = $(LOCALBIN)/golangci-lint

## Tool Versions
KUSTOMIZE_VERSION ?= v5.7.1
CONTROLLER_TOOLS_VERSION ?= v0.19.0

#ENVTEST_VERSION is the version of controller-runtime release branch to fetch the envtest setup script (i.e. release-0.20)
ENVTEST_VERSION ?= $(shell v='$(call gomodver,sigs.k8s.io/controller-runtime)'; \
  [ -n "$$v" ] || { echo "Set ENVTEST_VERSION manually (controller-runtime replace has no tag)" >&2; exit 1; }; \
  printf '%s\n' "$$v" | sed -E 's/^v?([0-9]+)\.([0-9]+).*/release-\1.\2/')

#ENVTEST_K8S_VERSION is the version of Kubernetes to use for setting up ENVTEST binaries (i.e. 1.31)
ENVTEST_K8S_VERSION ?= $(shell v='$(call gomodver,k8s.io/api)'; \
  [ -n "$$v" ] || { echo "Set ENVTEST_K8S_VERSION manually (k8s.io/api replace has no tag)" >&2; exit 1; }; \
  printf '%s\n' "$$v" | sed -E 's/^v?[0-9]+\.([0-9]+).*/1.\1/')

GOLANGCI_LINT_VERSION ?= v2.5.0
.PHONY: kustomize
kustomize: $(KUSTOMIZE) ## Download kustomize locally if necessary.
$(KUSTOMIZE): $(LOCALBIN)
	$(call go-install-tool,$(KUSTOMIZE),sigs.k8s.io/kustomize/kustomize/v5,$(KUSTOMIZE_VERSION))

.PHONY: controller-gen
controller-gen: $(CONTROLLER_GEN) ## Download controller-gen locally if necessary.
$(CONTROLLER_GEN): $(LOCALBIN)
	$(call go-install-tool,$(CONTROLLER_GEN),sigs.k8s.io/controller-tools/cmd/controller-gen,$(CONTROLLER_TOOLS_VERSION))

.PHONY: setup-envtest
setup-envtest: envtest ## Download the binaries required for ENVTEST in the local bin directory.
	@echo "Setting up envtest binaries for Kubernetes version $(ENVTEST_K8S_VERSION)..."
	@"$(ENVTEST)" use $(ENVTEST_K8S_VERSION) --bin-dir "$(LOCALBIN)" -p path || { \
		echo "Error: Failed to set up envtest binaries for version $(ENVTEST_K8S_VERSION)."; \
		exit 1; \
	}

.PHONY: envtest
envtest: $(ENVTEST) ## Download setup-envtest locally if necessary.
$(ENVTEST): $(LOCALBIN)
	$(call go-install-tool,$(ENVTEST),sigs.k8s.io/controller-runtime/tools/setup-envtest,$(ENVTEST_VERSION))

.PHONY: golangci-lint
golangci-lint: $(GOLANGCI_LINT) ## Download golangci-lint locally if necessary.
$(GOLANGCI_LINT): $(LOCALBIN)
	$(call go-install-tool,$(GOLANGCI_LINT),github.com/golangci/golangci-lint/v2/cmd/golangci-lint,$(GOLANGCI_LINT_VERSION))

# go-install-tool will 'go install' any package with custom target and name of binary, if it doesn't exist
# $1 - target path with name of binary
# $2 - package url which can be installed
# $3 - specific version of package
define go-install-tool
@[ -f "$(1)-$(3)" ] && [ "$$(readlink -- "$(1)" 2>/dev/null)" = "$(1)-$(3)" ] || { \
set -e; \
package=$(2)@$(3) ;\
echo "Downloading $${package}" ;\
rm -f "$(1)" ;\
GOBIN="$(LOCALBIN)" go install $${package} ;\
mv "$(LOCALBIN)/$$(basename "$(1)")" "$(1)-$(3)" ;\
} ;\
ln -sf "$$(realpath "$(1)-$(3)")" "$(1)"
endef

define gomodver
$(shell go list -m -f '{{if .Replace}}{{.Replace.Version}}{{else}}{{.Version}}{{end}}' $(1) 2>/dev/null)
endef
