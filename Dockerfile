# Build stage
FROM golang:1.25-alpine AS builder

RUN apk add --no-cache git ca-certificates tzdata

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /agenda ./cmd/server

# Runtime stage
FROM alpine:3.21

RUN apk add --no-cache ca-certificates tzdata

RUN addgroup -g 1000 -S nonroot && \
    adduser -u 1000 -S nonroot -G nonroot

COPY --from=builder --chown=nonroot:nonroot /agenda /agenda
COPY --chown=nonroot:nonroot migrations/ /migrations/

USER 1000

EXPOSE 8088

HEALTHCHECK --interval=30s --timeout=5s --start-period=5s --retries=3 \
  CMD wget -qO- http://localhost:8088/health || exit 1

ENTRYPOINT ["/agenda"]
