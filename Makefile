
# Image URL to use all building/pushing image targets
IMG ?= oamdev/terraform-controller:0.2.6

# Produce CRDs that work back to Kubernetes 1.11 (no version conversion)
CRD_OPTIONS ?= "crd:trivialVersions=true"

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

all: manager

# Run tests
test: generate fmt vet manifests
	go test ./controllers/... -coverprofile cover.out

# Build manager binary
manager: generate fmt vet
	go build -o bin/manager main.go

# Run against the configured Kubernetes cluster in ~/.kube/config
run: generate fmt vet manifests
	go run ./main.go

# Install CRDs into a cluster
install: manifests
	kustomize build chart/crds | kubectl apply -f -

# Uninstall CRDs from a cluster
uninstall: manifests
	kustomize build chart/crds | kubectl delete -f -

# Deploy controller in the configured Kubernetes cluster in ~/.kube/config
deploy: manifests
	cd config/manager && kustomize edit set image controller=${IMG}
	kustomize build config/default | kubectl apply -f -

# Generate manifests e.g. CRD, RBAC etc.
manifests: controller-gen
	$(CONTROLLER_GEN) $(CRD_OPTIONS) rbac:roleName=tf-api-role webhook paths="./..." output:crd:artifacts:config=chart/crds
	# mv config/rbac/role.yaml chart/templates/tf_api_role.yaml

# Run go fmt against code
fmt: goimports
	go fmt ./...

# Run go vet against code
vet:
	go vet ./...

# Generate code
generate: controller-gen
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./..."

# Build the docker image
docker-build: test
	docker build . -t ${IMG}

# Push the docker image
docker-push:
	docker push ${IMG}

# Make helm chart
chart: docker-build docker-push
	helm package chart --destination .

# find or download controller-gen
# download controller-gen if necessary
controller-gen:
ifeq (, $(shell which controller-gen))
	@{ \
	set -e ;\
	CONTROLLER_GEN_TMP_DIR=$$(mktemp -d) ;\
	cd $$CONTROLLER_GEN_TMP_DIR ;\
	go mod init tmp ;\
	go get sigs.k8s.io/controller-tools/cmd/controller-gen@v0.6.0 ;\
	rm -rf $$CONTROLLER_GEN_TMP_DIR ;\
	}
CONTROLLER_GEN=$(GOBIN)/controller-gen
else
CONTROLLER_GEN=$(shell which controller-gen)
endif

GOLANGCILINT_VERSION ?= v1.42.0
HOSTOS := $(shell uname -s | tr '[:upper:]' '[:lower:]')
HOSTARCH := $(shell uname -m)
ifeq ($(HOSTARCH),x86_64)
HOSTARCH := amd64
endif

golangci:
ifeq (, $(shell which golangci-lint))
	@{ \
	set -e ;\
	echo 'installing golangci-lint-$(GOLANGCILINT_VERSION)' ;\
	curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(GOBIN) $(GOLANGCILINT_VERSION) ;\
	echo 'Install succeed' ;\
	}
GOLANGCILINT=$(GOBIN)/golangci-lint
else
GOLANGCILINT=$(shell which golangci-lint)
endif

lint: golangci
	$(GOLANGCILINT) run ./...

reviewable: manifests fmt vet lint
	go mod tidy

# Execute auto-gen code commands and ensure branch is clean.
check-diff: reviewable
	git --no-pager diff
	git diff --quiet || ($(ERR) please run 'make reviewable' to include all changes && false)
	@$(OK) branch is clean

.PHONY: goimports
goimports:
ifeq (, $(shell which goimports))
	@{ \
	set -e ;\
	GO111MODULE=off go get -u golang.org/x/tools/cmd/goimports ;\
	}
GOIMPORTS=$(GOBIN)/goimports
else
GOIMPORTS=$(shell which goimports)
endif


install:
	helm lint ./chart
	helm upgrade --install --create-namespace --namespace terraform terraform-controller ./chart
	helm test -n terraform terraform-controller --timeout 5m

alibaba-credentials:
ifeq (, $(ALICLOUD_ACCESS_KEY))
	@echo "Environment variable ALICLOUD_ACCESS_KEY is not set"
	exit 1
endif

ifeq (, $(ALICLOUD_SECRET_KEY))
	@echo "Environment variable ALICLOUD_SECRET_KEY is not set"
	exit 1
endif

	echo "accessKeyID: ${ALICLOUD_ACCESS_KEY}\naccessKeySecret: ${ALICLOUD_SECRET_KEY}\nsecurityToken: ${ALICLOUD_SECURITY_TOKEN}" > alibaba-credentials.conf
	kubectl create namespace vela-system
	kubectl create secret generic alibaba-account-creds -n vela-system --from-file=credentials=alibaba-credentials.conf
	rm -f alibaba-credentials.conf
	kubectl get secret -n vela-system alibaba-account-creds

alibaba-provider:
	kubectl apply -f examples/alibaba/provider.yaml

alibaba: alibaba-credentials alibaba-provider


aws-credentials:
ifeq (, $(AWS_ACCESS_KEY_ID))
	@echo "Environment variable AWS_ACCESS_KEY_ID is not set"
	exit 1
endif

ifeq (, $(AWS_SECRET_ACCESS_KEY))
	@echo "Environment variable AWS_SECRET_ACCESS_KEY is not set"
	exit 1
endif

	# refer to https://registry.terraform.io/providers/hashicorp/aws/latest/docs
	echo "awsAccessKeyID: ${AWS_ACCESS_KEY_ID}\nawsSecretAccessKey: ${AWS_SECRET_ACCESS_KEY}\nawsSessionToken: ${AWS_SESSION_TOKEN}" > aws-credentials.conf
	kubectl create secret generic aws-account-creds -n vela-system --from-file=credentials=aws-credentials.conf
	rm -f aws-credentials.conf

aws-provider:
	kubectl apply -f examples/aws/provider.yaml

aws: aws-credentials aws-provider


azure-credentials:
ifeq (, $(ARM_CLIENT_ID))
	@echo "Environment variable ARM_CLIENT_ID is not set"
	exit 1
endif

ifeq (, $(ARM_CLIENT_SECRET))
	@echo "Environment variable ARM_CLIENT_SECRET is not set"
	exit 1
endif

ifeq (, $(ARM_SUBSCRIPTION_ID))
	@echo "Environment variable ARM_SUBSCRIPTION_ID is not set"
	exit 1
endif

ifeq (, $(ARM_TENANT_ID))
	@echo "Environment variable ARM_TENANT_ID is not set"
	exit 1
endif

	echo "armClientID: ${ARM_CLIENT_ID}\narmClientSecret: ${ARM_CLIENT_SECRET}\narmSubscriptionID: ${ARM_SUBSCRIPTION_ID}\narmTenantID: ${ARM_TENANT_ID}" > azure-credentials.conf
	kubectl create secret generic azure-account-creds -n vela-system --from-file=credentials=azure-credentials.conf
	rm -f azure-credentials.conf

azure-provider:
	kubectl apply -f examples/azure/provider.yaml

azure: azure-credentials azure-provider

configuration:
	go test -v ./e2e/...

e2e-setup: install alibaba

e2e: e2e-setup configuration
