DEPLOY_DIR = /home/ubuntu/CLIProxyAPIPlus

update-compose:
	cp compose.yml $(DEPLOY_DIR)/

update-env:
	cp .env.postgres.example $(DEPLOY_DIR)/.env

update-config:
	cp config.prod.yaml $(DEPLOY_DIR)/config.yaml


update-service:
	cd $(DEPLOY_DIR) && docker compose up -d

build:
	docker build --push --no-cache -t heishui/cli-proxy-api-plus:latest .

build-fast:
	docker build --push -t heishui/cli-proxy-api-plus:latest .

logs:
	cd $(DEPLOY_DIR) && docker compose logs cli-proxy-api

restart:
	cd $(DEPLOY_DIR) && docker compose down && docker compose up -d cli-proxy-api

status:
	cd $(DEPLOY_DIR) && docker compose ps

update: build-fast update-service