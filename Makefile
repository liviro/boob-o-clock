.PHONY: build test lint dev clean

# Build frontend + Go binary
build:
	cd web && npm run build
	go build -o boob-o-clock ./cmd/server

# Run all tests
test:
	go test ./...

# Lint frontend (react-hooks rules)
lint:
	cd web && npm run lint

# Dev mode: Go server on :8080, Vite dev server on :5173
dev:
	@echo "Starting Go server on :8080..."
	@go run ./cmd/server -addr :8080 &
	@echo "Starting Vite dev server on :5173..."
	@cd web && npm run dev

# Clean build artifacts
clean:
	rm -f boob-o-clock
	rm -rf internal/web/static/assets
