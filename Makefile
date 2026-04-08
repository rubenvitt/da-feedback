.PHONY: dev build docker migrate tailwind tailwind-watch

BINARY=da-feedback
TAILWIND=./tailwindcss

dev:
	@echo "Starting dev server..."
	@$(MAKE) tailwind-watch &
	@go run ./cmd/server -dev

build: tailwind
	CGO_ENABLED=0 go build -o $(BINARY) ./cmd/server

docker:
	docker build -t da-feedback .

migrate:
	go run ./cmd/server -migrate

tailwind:
	$(TAILWIND) -i static/input.css -o static/style.css --minify

tailwind-watch:
	$(TAILWIND) -i static/input.css -o static/style.css --watch

test:
	go test ./...
