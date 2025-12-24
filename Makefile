SHELL := /bin/bash
COMPOSE ?= docker compose
LAB_COMPOSE := docker/lab/docker-compose.yml

.PHONY: lab-all lab-up lab-test lab-down

lab-all:
	scripts/lab-all.sh

lab-up:
	docker/lab/scripts/gen-secrets.sh
	$(COMPOSE) -f $(LAB_COMPOSE) up -d panel agent-mtls agent-jwt

lab-test:
	$(COMPOSE) -f $(LAB_COMPOSE) up --build --abort-on-container-exit --exit-code-from e2e e2e

lab-down:
	$(COMPOSE) -f $(LAB_COMPOSE) down -v
