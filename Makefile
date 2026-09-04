ifneq (,$(wildcard .env))
include .env
endif

DOCKER ?= docker
GO ?= go
LDFLAGS ?= -s -w

IMAGE ?= docker.io/library/goont
TAG ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)

DIST_DIR ?= dist
BIN_DIR ?= bin
TAR_FILE ?= $(DIST_DIR)/goont-$(TAG).tar.gz

CONTAINER_NAME ?= goont
ENV_FILE ?= .env
ENVFILE = $(if $(wildcard $(ENV_FILE)),--env-file $(ENV_FILE),)

.DEFAULT_GOAL := help
.PHONY: help build build-cli build-server check fmt vet tidy image save run-server run-scan run-cli stop restart logs shell clean

help:
	@echo "GoONT - build, ejecucion y despliegue"
	@echo ""
	@echo "Calidad y build local:"
	@echo "  make check                        gofmt + vet + compila los dos binarios"
	@echo "  make build-cli                    compila bin/goont"
	@echo "  make build-server                 compila bin/goont-server"
	@echo "  make tidy                         go mod tidy"
	@echo ""
	@echo "Imagen Docker:"
	@echo "  make image                        construye la imagen goont:$(TAG)"
	@echo "  make save                         image + exporta $(TAR_FILE) (para copiar al servidor)"
	@echo ""
	@echo "Ejecucion con Docker (toma variables de .env, mira .env.example):"
	@echo "  make run-server                   levanta el servidor API en segundo plano"
	@echo "  make run-scan                     ejecuta 'ont scan' (contenedor efimero)"
	@echo "  make run-cli CMD='olt list'       ejecuta cualquier comando de la CLI"
	@echo "  make stop / restart / logs / shell"
	@echo ""
	@echo "  make clean                        borra dist/ y bin/"

build: build-cli build-server

build-cli:
	$(GO) build -trimpath -ldflags="$(LDFLAGS)" -o $(BIN_DIR)/goont ./cmd/cli

build-server:
	$(GO) build -trimpath -ldflags="$(LDFLAGS)" -o $(BIN_DIR)/goont-server ./cmd/server

check: fmt vet build

fmt:
	@gofmt -w .

vet:
	@$(GO) vet ./...

tidy:
	@$(GO) mod tidy

image:
	$(DOCKER) build -t $(IMAGE):$(TAG) -t $(IMAGE):latest .

save: image
	@mkdir -p $(DIST_DIR)
	$(DOCKER) save $(IMAGE):$(TAG) | gzip > $(TAR_FILE)
	@ls -lh $(TAR_FILE)

run-server:
	$(DOCKER) run -d --name $(CONTAINER_NAME) --restart unless-stopped --network host \
		--health-cmd 'wget -qO- http://127.0.0.1:8080/api/v1/health >/dev/null 2>&1 || exit 1' \
		--health-interval=30s --health-timeout=5s --health-retries=3 --health-start-period=10s \
		$(ENVFILE) \
		$(IMAGE):$(TAG)

run-scan:
	$(DOCKER) run --rm --network host $(ENVFILE) $(IMAGE):$(TAG) goont ont scan

run-cli:
	@test -n "$(CMD)" || { echo "uso: make run-cli CMD='olt list'"; exit 1; }
	$(DOCKER) run --rm --network host $(ENVFILE) $(IMAGE):$(TAG) goont $(CMD)

stop:
	@$(DOCKER) rm -f $(CONTAINER_NAME) 2>/dev/null || true

restart:
	@$(MAKE) stop
	@$(MAKE) run-server

logs:
	$(DOCKER) logs --tail 100 -f $(CONTAINER_NAME)

shell:
	$(DOCKER) exec -it $(CONTAINER_NAME) sh

clean:
	rm -rf $(DIST_DIR) $(BIN_DIR)
