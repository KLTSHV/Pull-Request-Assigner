APP_NAME=pr-reviewer
CMD_PATH=./cmd/app

.PHONY: all build run test clean docker-build docker-run

all: build

build:
	go build -o bin/$(APP_NAME) $(CMD_PATH)

run:
	go run $(CMD_PATH)

test:
	go test ./...

clean:
	rm -rf bin

docker-build:
	docker build -t $(APP_NAME) .

docker-run:
	docker run --rm \
		-p 8080:8080 \
		-e HTTP_PORT=8080 \
		-e DB_DSN="postgres://postgres:postgres@db:5432/postgres?sslmode=disable" \
		$(APP_NAME)
