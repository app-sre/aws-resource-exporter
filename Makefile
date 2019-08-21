.PHONY: all build image check vendor dependencies
NAME				:= aws-resource-exporter
REPO				:= quay.io/app-sre/$(NAME)
TAG					:= $(shell git rev-parse --short HEAD)

PKGS				:= $(shell go list ./... | grep -v -E '/vendor/|/test')
FIRST_GOPATH		:= $(firstword $(subst :, ,$(shell go env GOPATH)))
GOLANGCI_LINT_BIN	:= $(FIRST_GOPATH)/bin/golangci-lint

.PHONY: all
all: build

.PHONY: clean
clean:
	# Remove all files and directories ignored by git.
	git clean -Xfd .

############
# Building #
############

build:
	go build -o $(NAME) .

vendor:
	go mod tidy
	go mod vendor
	go mod verify

image: build
	docker build -t $(REPO):$(TAG) .

image-push:
	docker push $(REPO):$(TAG)

##############
# Formatting #
##############

.PHONY: lint
lint: $(GOLANGCI_LINT_BIN)
	# megacheck fails to respect build flags, causing compilation failure during linting.
	# instead, use the unused, gosimple, and staticcheck linters directly
	$(GOLANGCI_LINT_BIN) run -D megacheck -E unused,gosimple,staticcheck

.PHONY: format
format: go-fmt

.PHONY: go-fmt
go-fmt:
	go fmt $(PKGS)

###########
# Testing #
###########

.PHONY: test
test: test-unit

.PHONY: test-unit
test-unit:
	go test -race -short $(PKGS) -count=1

############
# Binaries #
############

dependencies: $(GOLANGCI_LINT_BIN)

$(GOLANGCI_LINT_BIN):
	curl -sfL https://install.goreleaser.com/github.com/golangci/golangci-lint.sh | sh -s -- -b $(FIRST_GOPATH)/bin v1.16.0

