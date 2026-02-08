SWAG ?= swag

.PHONY: docs api compose dev test

docs:
	$(SWAG) init -g inputs/api/main.go -o docs || \
		go run github.com/swaggo/swag/cmd/swag@v1.16.6 init -g inputs/api/main.go -o docs

api:
	go run .

compose:
	docker compose up -d

test:
	go test ./...

dev: compose docs api
