.PHONY: check clean build image publish publish-latest render display test

TAG_NAME := $(shell git tag -l --contains HEAD)
SHA := $(shell git rev-parse --short HEAD)
VERSION := $(if $(TAG_NAME),$(TAG_NAME),$(SHA))
BUILD_DATE := $(shell date -u '+%Y-%m-%d_%I:%M:%S%p')
DOCKER_REGISTRY := gcr.io
DOCKER_REPOSITORY := trois-six/k8s-diagrams
OUTPUT_DIR ?= diagrams
NAMESPACE ?= traefikee
VIEWER ?= feh

default: clean build render

check:
	@golangci-lint run

clean:
	@rm -rf $(OUTPUT_DIR)

build: clean
	@echo Version: $(VERSION) $(BUILD_DATE)
	CGO_ENABLED=0 go build -v -ldflags '-X "main.version=${VERSION}" -X "main.commit=${SHA}" -X "main.date=${BUILD_DATE}"'

image:
	docker build -t $(DOCKER_REGISTRY)/$(DOCKER_REPOSITORY):$(VERSION) .

publish:
	docker push $(DOCKER_REGISTRY)/$(DOCKER_REPOSITORY):$(VERSION)

publish-latest:
	docker tag $(DOCKER_REGISTRY)/$(DOCKER_REPOSITORY):$(VERSION) $(DOCKER_REGISTRY)/$(DOCKER_REPOSITORY):latest
	docker push $(DOCKER_REGISTRY)/$(DOCKER_REPOSITORY):latest

render: clean
	@./k8s-diagrams -n $(NAMESPACE) -d $(OUTPUT_DIR)
	@cd $(OUTPUT_DIR) && dot -Tpng k8s.dot > ../diagram.png

display: default
	@$(VIEWER) diagram.png

test: clean
	go test -v -cover ./...
