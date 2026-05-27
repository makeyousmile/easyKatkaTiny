FROM tinygo/tinygo:0.41.1 AS builder
WORKDIR /src
COPY go.mod ./
COPY cmd ./cmd
COPY internal ./internal
RUN tinygo build -o /out/app ./cmd/easykatka

FROM alpine:3.19
WORKDIR /app
RUN apk add --no-cache curl ca-certificates
COPY --from=builder /out/app /app/app
COPY account_id /app/account_id
CMD ["/app/app"]
