include .env

MIGRATE_IMAGE=migrate/migrate
MIGRATIONS_PATH=backend/migrations
GO_IMAGE=golang:1.25-alpine
COMPOSE_NETWORK=dentvision-ai_default
TEST_DATABASE_URL=postgres://$(POSTGRES_USER):$(POSTGRES_PASSWORD)@postgres:5432/$(POSTGRES_DB)?sslmode=disable

up:
	docker compose up -d

down:
	docker compose down

build:
	docker compose up -d --build

logs:
	docker compose logs -f

logs-backend:
	docker compose logs -f backend

logs-ml:
	docker compose logs -f ml-service

logs-frontend:
	docker compose logs -f frontend

ps:
	docker ps

migrate-create:
	docker run --rm -v $(PWD)/$(MIGRATIONS_PATH):/migrations $(MIGRATE_IMAGE) create -ext sql -dir /migrations -seq $(name)

migrate-up:
	docker run --rm -v $(PWD)/$(MIGRATIONS_PATH):/migrations --network host $(MIGRATE_IMAGE) -path /migrations -database "$(MIGRATE_DATABASE_URL)" up

migrate-down:
	docker run --rm -v $(PWD)/$(MIGRATIONS_PATH):/migrations --network host $(MIGRATE_IMAGE) -path /migrations -database "$(MIGRATE_DATABASE_URL)" down 1

migrate-force:
	docker run --rm -v $(PWD)/$(MIGRATIONS_PATH):/migrations --network host $(MIGRATE_IMAGE) -path /migrations -database "$(MIGRATE_DATABASE_URL)" force $(version)

test-backend:
	docker run --rm -v $(PWD)/backend:/app -w /app --network $(COMPOSE_NETWORK) -e TEST_DATABASE_URL="$(TEST_DATABASE_URL)" $(GO_IMAGE) go test ./...
