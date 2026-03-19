FROM golang:1.22-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -o /server ./cmd/server

FROM alpine:3.19

RUN apk add --no-cache ca-certificates \
    && addgroup -g 1001 -S appgroup \
    && adduser -S appuser -u 1001 -G appgroup

COPY --from=builder --chown=appuser:appgroup /server /server
COPY --from=builder --chown=appuser:appgroup /app/migrations /migrations

USER appuser

EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=3s CMD wget -qO- http://localhost:8080/health || exit 1

ENTRYPOINT ["/server"]
