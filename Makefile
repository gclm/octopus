.PHONY: dev dev-fe dev-be build build-fe build-be run clean

# Dev: run frontend and backend separately with hot reload
dev: dev-be dev-fe

# Dev frontend (Next.js dev server, port 3000)
dev-fe:
	cd web && pnpm dev

# Dev backend (Go with debug mode, port 8080)
dev-be:
	go run . start

# Build everything (frontend + backend)
build: build-fe build-be

# Build frontend and copy to static/
build-fe:
	cd web && pnpm install && pnpm build
	rm -rf static/out
	cp -r web/out static/out

# Build Go binary (embeds static/out)
build-be: build-fe
	go build -o octopus .

# Run the built binary
run: build
	./octopus start

# Clean build artifacts
clean:
	rm -rf static/out web/out web/.next octopus
