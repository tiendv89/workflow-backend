.PHONY: run-api migrate-up migrate-down-1 new-migration lint test

# run-api: will use cobra entrypoint (T4); for now runs cmd/api-service directly
run-api:
	go run ./cmd/api-service

# migrate-up: will use cobra entrypoint (T4); for now uses database.RunMigrations via api startup
migrate-up:
	go run ./cmd/api-service

# migrate-down-1: stub until cobra migration subcommand lands in T4
migrate-down-1:
	@echo "migrate-down-1 requires the cobra migration subcommand (T4)"
	@exit 1

# new-migration: create a new goose migration file
new-migration:
	@test -n "$(NAME)" || (echo "usage: make new-migration NAME=<name>" && exit 1)
	goose -dir migrations create $(NAME) sql

lint:
	golangci-lint run

test:
	go test ./... -race
