# Stage 1: build Go backend
FROM golang:1.25-alpine AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY cmd ./cmd
COPY internal ./internal
COPY VERSION ./VERSION

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/notediscovery ./cmd/notediscovery

# Stage 2: minimal runtime
FROM alpine:3.22

WORKDIR /app

RUN apk add --no-cache ca-certificates

COPY --from=builder /out/notediscovery /app/notediscovery
COPY frontend ./frontend
COPY config.yaml ./config.yaml
COPY VERSION ./VERSION
COPY plugins ./plugins
COPY themes ./themes
COPY locales ./locales

RUN mkdir -p /app/data

EXPOSE 8000
ENV PORT=8000

HEALTHCHECK --interval=60s --timeout=3s --start-period=5s --retries=3 \
  CMD wget -qO- "http://localhost:${PORT}/health" >/dev/null || exit 1

CMD ["/app/notediscovery", "-config", "/app/config.yaml"]
