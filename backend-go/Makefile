APP        := oriva-backend
PKG        := oriva/backend-go
VERSION    ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT     ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
DATE       := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS    := -X '$(PKG)/utils/buildinfo.Version=$(VERSION)' \
              -X '$(PKG)/utils/buildinfo.Commit=$(COMMIT)' \
              -X '$(PKG)/utils/buildinfo.Date=$(DATE)'

TEST_DATABASE_URL ?= postgres://oriva:oriva@localhost:5432/oriva_test?sslmode=disable

.PHONY: run build test test-integration lint tidy \
        db-up db-down migrate-up migrate-down migrate-version migrate-create seed

run:
	go run ./cmd/server

build:
	go build -ldflags "$(LDFLAGS)" -o bin/$(APP) ./cmd/server

test:
	go test ./...

test-integration:
	TEST_DATABASE_URL="$(TEST_DATABASE_URL)" go test -tags=integration ./...

tidy:
	go mod tidy

db-up:
	docker compose up -d postgres

db-down:
	docker compose down

migrate-up:
	go run ./cmd/migrate up

migrate-down:
	go run ./cmd/migrate down

migrate-version:
	go run ./cmd/migrate version

# make migrate-create name=add_widgets
migrate-create:
	@test -n "$(name)" || (echo "usage: make migrate-create name=<slug>"; exit 1)
	@next=$$(printf '%04d' $$(( $$(ls migrations/*.up.sql 2>/dev/null | wc -l) + 1 ))); \
	touch migrations/$${next}_$(name).up.sql migrations/$${next}_$(name).down.sql; \
	echo "created migrations/$${next}_$(name).{up,down}.sql"

seed:
	go run ./cmd/seed
