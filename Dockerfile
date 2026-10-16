FROM golang:alpine AS builder
WORKDIR /src
COPY go.mod go.sum* ./
COPY vendor ./vendor
COPY cmd ./cmd
COPY internal ./internal
RUN CGO_ENABLED=0 go build -mod=vendor -ldflags="-s -w" -o app ./cmd/easykatka

FROM alpine:3.19
WORKDIR /app
RUN apk add --no-cache ca-certificates tzdata
COPY --from=builder /src/app /app/app
COPY account_id /app/account_id
COPY images /app/images
CMD ["/app/app"]
