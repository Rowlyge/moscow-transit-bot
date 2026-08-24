FROM golang:1.26.5-bookworm AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .

RUN CGO_ENABLED=0 go build -o /out/etl ./cmd/etl
RUN CGO_ENABLED=0 go build -o /out/bot ./cmd/bot

FROM alpine:3.20 AS etl
RUN apk add --no-cache ca-certificates
COPY --from=builder /out/etl /usr/local/bin/etl
ENTRYPOINT ["/usr/local/bin/etl"]

FROM alpine:3.20 AS bot
RUN apk add --no-cache ca-certificates
COPY --from=builder /out/bot /usr/local/bin/bot
ENTRYPOINT ["/usr/local/bin/bot"]
