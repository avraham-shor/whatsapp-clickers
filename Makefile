# bash recipes (`wait -n`); on Windows run under Git Bash (or `choco install make`).
# No-make fallback: `go run ./cmd/server` in /server and `npm run dev` in /web.

SHELL := bash

.PHONY: dev build test generate lint clean

## dev: run Go API (:8080) and Vite dev server (:5173) concurrently.
## Exits non-zero as soon as either process dies (e.g. missing env vars).
dev:
	(cd server && go run ./cmd/server) & (cd web && npm run dev) & \
	wait -n; status=$$?; kill $$(jobs -p) 2>/dev/null; exit $$status

## build: frontend build -> embed into server -> single binary at server/bin/server
build:
	cd web && npm run build
	rm -rf server/internal/webdist/dist
	mkdir -p server/internal/webdist/dist
	cp -r web/dist/. server/internal/webdist/dist/
	cd server && go build -o bin/server ./cmd/server

## test: backend + frontend test suites
test:
	cd server && go test ./...
	cd web && npm test

## generate: regenerate sqlc output (commit the result)
generate:
	cd server && sqlc generate

## lint: all static checks that CI runs
lint:
	cd server && go vet ./...
	cd server && test -z "$$(gofmt -l .)" || (gofmt -l . && exit 1)
	cd web && npm run lint
	cd web && npx tsc -b --noEmit

## clean: remove build output and restore the committed webdist placeholder
clean:
	rm -rf server/bin web/dist server/internal/webdist/dist
	git checkout -- server/internal/webdist/dist
