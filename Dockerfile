FROM golang:1.22-alpine AS builder

WORKDIR /app

RUN apk add --no-cache ca-certificates

COPY go.mod go.sum ./
RUN go mod download

COPY . .


RUN CGO_ENABLED=0 GOOS=linux go build -o pr-reviewer ./cmd/app

FROM alpine:3.20

RUN apk add --no-cache ca-certificates

WORKDIR /app

COPY --from=builder /app/pr-reviewer /app/pr-reviewer

ENV HTTP_PORT=8080
ENV DB_DSN=postgres://postgres:postgres@db:5432/postgres?sslmode=disable

EXPOSE 8080

CMD ["/app/pr-reviewer"]
