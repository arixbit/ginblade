# syntax=docker/dockerfile:1.7

FROM golang:1.26.5-alpine AS builder

ENV CGO_ENABLED=0 \
    GOTOOLCHAIN=local

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go build -buildvcs=false -trimpath -ldflags="-s -w" -o /out/api ./cmd/api
RUN go build -buildvcs=false -trimpath -ldflags="-s -w" -o /out/worker ./cmd/worker
RUN go build -buildvcs=false -trimpath -ldflags="-s -w" -o /out/migrate ./cmd/migrate

FROM alpine:3.22

RUN apk add --no-cache ca-certificates tzdata \
    && addgroup -S app \
    && adduser -S -G app app

COPY --from=builder --chown=app:app /out/ /usr/local/bin/

USER app
EXPOSE 3000

CMD ["/usr/local/bin/api"]
