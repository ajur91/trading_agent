FROM golang:1.21-alpine AS builder

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /market-api ./cmd/main.go

FROM alpine:3.19
RUN apk add --no-cache ca-certificates tzdata wget
WORKDIR /app
COPY --from=builder /market-api .
EXPOSE 8080
HEALTHCHECK --interval=15s --timeout=10s --retries=8 --start-period=10s \
    CMD wget -qO- http://localhost:${API_PORT:-8080}/health
CMD ["./market-api"]
