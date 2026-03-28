APP_NAME := hearthstone-analyzer
APP_IMAGE := $(APP_NAME):dev
APP_PORT := 8080
DATA_DIR := $(CURDIR)/data

.PHONY: test
test:
	go test ./...

.PHONY: build
build:
	go build -o bin/$(APP_NAME).exe ./cmd/api

.PHONY: run
run:
	go run ./cmd/api

.PHONY: frontend-install
frontend-install:
	cd web && npm install

.PHONY: frontend-test
frontend-test:
	cd web && npm test

.PHONY: frontend-build
frontend-build:
	cd web && npm run build

.PHONY: verify
verify: frontend-test frontend-build test

.PHONY: docker-build
docker-build:
	docker build -t $(APP_IMAGE) .

.PHONY: docker-run
docker-run:
	docker run --rm -p $(APP_PORT):8080 -v "$(DATA_DIR):/data" $(APP_IMAGE)

.PHONY: smoke-health
smoke-health:
	curl -fsS http://localhost:$(APP_PORT)/healthz

.PHONY: smoke-api
smoke-api:
	curl -fsS http://localhost:$(APP_PORT)/healthz
	curl -fsS http://localhost:$(APP_PORT)/api/jobs > /dev/null
	curl -fsS http://localhost:$(APP_PORT)/api/reports > /dev/null
	curl -fsS http://localhost:$(APP_PORT)/api/settings > /dev/null
	curl -fsS http://localhost:$(APP_PORT)/api/meta/latest?format=standard > /dev/null || true
