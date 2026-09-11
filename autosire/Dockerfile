# Build Stage
FROM golang:alpine AS builder
WORKDIR /app
COPY go.mod ./
COPY go.sum* ./
RUN go mod download || true
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /app/autosire-server ./cmd/server

# Final Lightweight Runtime Stage
FROM alpine:latest
RUN apk --no-cache add ca-certificates tzdata
WORKDIR /app
COPY --from=builder /app/autosire-server /app/autosire-server
COPY web/static /app/web/static
VOLUME /app/data
EXPOSE 8090
ENV PORT=8090 DATA_DIR=/app/data SECRET_KEY=AUTOSIRE-SECRET-2026-VPS-KEY
ENTRYPOINT ["/app/autosire-server"]
