.PHONY: up down logs fmt lint build test check tidy

# Сборка и запуск сервиса + БД
up:
	docker compose up --build

# Остановка и удаление контейнеров вместе с данными БД
down:
	docker compose down -v

# Логи приложения
logs:
	docker compose logs -f app

# Авто-форматирование кода
fmt:
	gofmt -w .

# Линтер (включает go vet, staticcheck, errcheck и др.)
lint:
	golangci-lint run ./...

# Компиляция
build:
	go build ./...

# Тесты без кэша
test:
	go test -count=1 ./...

# Все проверки разом — зеркало pre-commit хука
check:
	@gofmt -l . | grep . && echo "не отформатированы, запустите: make fmt" && exit 1 || true
	golangci-lint run ./...
	go build ./...
	go test -count=1 ./...

# Обновление go.mod / go.sum
tidy:
	go mod tidy
