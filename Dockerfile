FROM golang:1.26.4-alpine AS builder

WORKDIR /app
RUN apk add --no-cache ca-certificates tzdata

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o /app/binary-api ./cmd/api

FROM alpine:3.21
RUN apk add --no-cache ca-certificates tzdata wget && adduser -D -H -u 10001 appuser
WORKDIR /app
COPY --from=builder /app/binary-api /app/binary-api
USER appuser
EXPOSE 8080

HEALTHCHECK --interval=10s --timeout=3s --start-period=5s --retries=3 CMD wget -qO- http://localhost:8080/health || exit 1
ENTRYPOINT ["/app/binary-api"]
