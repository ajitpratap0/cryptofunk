.PHONY: up down restart build migrate logs status tf-init tf-plan tf-apply

COMPOSE := docker-compose

## Docker Compose
up:
	$(COMPOSE) up -d

down:
	$(COMPOSE) down

restart:
	$(COMPOSE) down
	$(COMPOSE) up -d

build:
	$(COMPOSE) build

migrate:
	$(COMPOSE) run --rm migrate up

logs:
	$(COMPOSE) logs -f --tail=100

status:
	$(COMPOSE) ps
	@echo ""
	@echo "URLs:"
	@echo "  Dashboard:  http://localhost:3001"
	@echo "  API:        http://localhost:8082"
	@echo "  Grafana:    http://localhost:3000"
	@echo "  Prometheus: http://localhost:9090"

## Terraform
tf-init:
	cd infra/terraform && terraform init

tf-plan:
	cd infra/terraform && terraform plan

tf-apply:
	cd infra/terraform && terraform apply
