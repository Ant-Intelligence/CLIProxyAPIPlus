DEPLOY_DIR = /root/CLIProxyAPIPlus
FRONTEND_DIR = frontend
STATIC_DIR = static

# ── Frontend ──────────────────────────────────────

frontend-deps:
	cd $(FRONTEND_DIR) && npm ci

frontend-build: frontend-deps
	cd $(FRONTEND_DIR) && npm run build
	mkdir -p $(STATIC_DIR)
	cp $(FRONTEND_DIR)/dist/index.html $(STATIC_DIR)/management.html

frontend-dev:
	cd $(FRONTEND_DIR) && npm run dev

frontend-update:
	git submodule update --remote --merge $(FRONTEND_DIR)

frontend-clean:
	rm -rf $(FRONTEND_DIR)/node_modules $(FRONTEND_DIR)/dist $(STATIC_DIR)/management.html

# ── Backend ───────────────────────────────────────

server-build:
	go build -o CLIProxyAPIPlus ./cmd/server

client-build:
	go build -o cpa-client ./cmd/client

server-run: server-build
	./CLIProxyAPIPlus

# ── Combined ──────────────────────────────────────

dev: frontend-build server-run

# ── Deploy ────────────────────────────────────────

update-compose:
	cp compose.yml $(DEPLOY_DIR)/

update-env:
	cp .env.postgres.example $(DEPLOY_DIR)/.env

update-config:
	cp config.prod.yaml $(DEPLOY_DIR)/config.yaml

update-service:
	cd $(DEPLOY_DIR) && docker compose up -d

build:
	docker build --push --no-cache -t heishui/cli-proxy-api-plus:cc-monitor .

build-fast:
	docker build --push -t heishui/cli-proxy-api-plus:cc-monitor .

logs:
	cd $(DEPLOY_DIR) && docker compose logs cli-proxy-api

restart:
	cd $(DEPLOY_DIR) && docker compose down && docker compose up -d cli-proxy-api

status:
	cd $(DEPLOY_DIR) && docker compose ps

update: build-fast update-service