# Stage 1: build the SPA
FROM node:22-alpine AS web
WORKDIR /src/web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

# Stage 2: build the Go binary with the SPA embedded
FROM golang:1.26.5-alpine AS server
WORKDIR /src/server
COPY server/go.mod server/go.sum ./
RUN go mod download
COPY server/ ./
COPY --from=web /src/web/dist ./internal/webdist/dist
RUN CGO_ENABLED=0 go build -o /out/server ./cmd/server

# Stage 3: minimal runtime
FROM alpine:3.21
RUN apk add --no-cache ca-certificates tzdata
COPY --from=server /out/server /app/server
EXPOSE 8080
CMD ["/app/server"]
