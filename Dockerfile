FROM golang:1.23-alpine AS builder

WORKDIR /app

COPY go.mod .
COPY bridge/ ./bridge
COPY main.go .

RUN go mod download
RUN go build -o /app/main

FROM alpine:3.18

COPY --from=builder /app/main /app/main

ENTRYPOINT ["/app/main"]