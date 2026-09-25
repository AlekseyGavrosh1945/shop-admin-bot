BINARY := shop-admin-bot

.PHONY: run build test vet tidy up down logs

## run: запустить бота локально (нужен MySQL и .env)
run:
	go run ./cmd/bot

## build: собрать бинарник
build:
	CGO_ENABLED=0 go build -o $(BINARY) ./cmd/bot

## test: прогнать тесты (TEST_DSN включает интеграционные SQL-тесты)
test:
	go test -race ./...

## vet: статический анализ
vet:
	go vet ./...

## tidy: почистить зависимости
tidy:
	go mod tidy

## up: поднять бота в Docker (БД — вне контейнера, см. .env)
up:
	docker compose up -d --build

## down: остановить контейнер
down:
	docker compose down

## logs: следить за логами
logs:
	docker compose logs -f bot
