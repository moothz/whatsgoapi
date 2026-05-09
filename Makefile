.PHONY: up down restart logs build help

help:
	@echo "WhatsGoAPI Standalone Management"
	@echo "  make up      - Start the API and dependencies"
	@echo "  make down    - Stop the API and dependencies"
	@echo "  make down-v  - Stop and REMOVE volumes (resets DB)"
	@echo "  make restart - Restart the API"
	@echo "  make logs    - Show logs"
	@echo "  make build   - Rebuild the API image"
	@echo "  make setup   - Generate .env with random secrets"
	@echo "  make docs    - Generate Swagger documentation"

setup:
	@if [ ! -f .env ]; then \
		cp .env.example .env; \
		G_KEY=$$(tr -dc 'a-zA-Z0-9' < /dev/urandom | fold -w 30 | head -n 1); \
		P_PASS=$$(openssl rand -hex 16); \
		M_KEY=$$(openssl rand -hex 16); \
		sed -i "s/GLOBAL_API_KEY=30CHARRANDOMSTRING/GLOBAL_API_KEY=$$G_KEY/" .env; \
		sed -i "s/WHATS_GO_API_KEY=30CHARRANDOMSTRING/WHATS_GO_API_KEY=$$G_KEY/" .env; \
		sed -i "s/POSTGRES_PASSWORD=password/POSTGRES_PASSWORD=$$P_PASS/" .env; \
		sed -i "s/MINIO_SECRET_KEY=minioadmin/MINIO_SECRET_KEY=$$M_KEY/" .env; \
		echo ".env generated with random secrets (API Key: 30 chars)."; \
	else \
		echo ".env already exists. Skipping setup."; \
	fi

up:
	docker compose up -d

up-build:
	docker compose up -d --build

down:
	docker compose down --remove-orphans

down-v:
	docker compose down -v --remove-orphans

restart:
	docker compose restart

logs:
	docker compose logs -f whatsgoapi

build:
	docker compose build whatsgoapi

docs:
	swag init -g cmd/whatsgo/main.go
