NAME				:= aws-resource-exporter
REPO				:= quay.io/app-sre/$(NAME)
TAG					:= $(shell git rev-parse --short HEAD)

PKGS				:= $(shell go list ./... | grep -v -E '/vendor/|/test')
FIRST_GOPATH		:= $(firstword $(subst :, ,$(shell go env GOPATH)))
CONTAINER_ENGINE    ?= $(shell which podman >/dev/null 2>&1 && echo podman || echo docker)

ifneq (,$(wildcard $(CURDIR)/.docker))
	DOCKER_CONF := $(CURDIR)/.docker
else
	DOCKER_CONF := $(HOME)/.docker
endif

.PHONY: all
all: test image

.PHONY: clean
clean:
	# Remove all files and directories ignored by git.
	git clean -Xfd .

############
# Building #
############

.PHONY: build
build:
	go build -o $(NAME) .

.PHONY: image
image:
	$(CONTAINER_ENGINE) build -t $(REPO):$(TAG) .

.PHONY: image-push
image-push:
	$(CONTAINER_ENGINE) tag $(REPO):$(TAG) $(REPO):latest
	$(CONTAINER_ENGINE) --config=$(DOCKER_CONF) push $(REPO):$(TAG)
	$(CONTAINER_ENGINE) --config=$(DOCKER_CONF) push $(REPO):latest

##############
# Formatting #
##############

.PHONY: format
format: go-fmt

.PHONY: go-fmt
go-fmt:
	go fmt $(PKGS)

###########
# Testing #
###########

.PHONY: vet
vet:
	go vet ./...

.PHONY: test
test: vet test-unit

.PHONY: test-unit
test-unit:
	go test -race -short $(PKGS) -count=1

