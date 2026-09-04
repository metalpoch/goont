FROM golang:1.25-alpine AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/goont ./cmd/cli && \
    CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/goont-server ./cmd/server

FROM alpine:3.22

RUN apk add --no-cache ca-certificates && \
    adduser -D -H -u 1000 goont

COPY --from=builder /out/goont /usr/local/bin/goont
COPY --from=builder /out/goont-server /usr/local/bin/goont-server

USER goont

EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD wget -qO- http://127.0.0.1:8080/api/v1/health >/dev/null 2>&1 || exit 1

CMD ["/usr/local/bin/goont-server"]
