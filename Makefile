.PHONY: run-api migrate-up migrate-down-1 new-migration lint test

run-api:
	go run ./cmd -c configs/config.yaml api

migrate-up:
	go run ./cmd -c configs/config.yaml migration -u 0

migrate-down-1:
	go run ./cmd -c configs/config.yaml migration -d 1

new-migration:
	@test -n "$(NAME)" || (echo "usage: make new-migration NAME=<name>" && exit 1)
	goose -dir migrations create $(NAME) sql

lint:
	golangci-lint run

test:
	go test ./... -race
