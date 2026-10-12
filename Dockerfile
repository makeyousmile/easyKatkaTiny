FROM golang:1.22-alpine AS builder
RUN apk add --no-cache gcc musl-dev
WORKDIR /src
COPY go.mod ./
COPY cmd ./cmd
COPY internal ./internal
RUN CGO_ENABLED=1 go build -o app ./cmd/easykatka

FROM alpine:3.19
WORKDIR /app
RUN apk add --no-cache curl ca-certificates
COPY --from=builder /src/app /app/app
COPY account_id /app/account_id
CMD ["/app/app"]
