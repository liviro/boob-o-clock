# Stage 1: Build frontend
FROM node:22-alpine AS frontend
WORKDIR /app/web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ ./
COPY internal/web/static/ ../internal/web/static/
RUN npm run build

# Stage 2: Build Go binary
FROM golang:1.25-alpine AS backend
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=frontend /app/internal/web/static/ ./internal/web/static/
RUN CGO_ENABLED=0 go build -o /boob-o-clock ./cmd/server

# Stage 3: Final image
FROM alpine:3.21
RUN apk add --no-cache ca-certificates tzdata
COPY --from=backend /boob-o-clock /usr/local/bin/boob-o-clock

VOLUME /data
ENV PORT=8080
EXPOSE ${PORT}
HEALTHCHECK --interval=30s --timeout=3s --retries=3 \
  CMD wget -qO- http://localhost:${PORT}/healthz || exit 1
ENTRYPOINT ["boob-o-clock"]
CMD ["-db", "/data/boob-o-clock.db"]
