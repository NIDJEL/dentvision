include .env

MIGRATE_IMAGE=migrate/migrate
MIGRATIONS_PATH=backend/migrations

up:
	docker compose up -d

down:
	docker compose down

build:
	docker compose up -d --build

logs:
	docker compose logs -f

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