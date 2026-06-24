.PHONY: up down logs fmt lint build test test-integration check tidy test-docker test-integration-docker

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

# Интеграционные тесты с живой БД. Требует DATABASE_URL.
test-integration:
	docker compose up -d db
	@test -n "$$DATABASE_URL" || (echo "DATABASE_URL is required"; exit 1)
	RUN_INTEGRATION=1 go test -count=1 -run 'TestApply_' ./internal/repository

# Unit/API тесты внутри app-контейнера
test-docker:
	docker compose run --build --rm app go test -count=1 ./...

# Интеграционные тесты внутри app-контейнера с БД из docker compose
test-integration-docker:
	docker compose up -d db
	@until docker compose exec -T db sh -c 'pg_isready -U "$$POSTGRES_USER" -d "$$POSTGRES_DB"' >/dev/null 2>&1; do \
 		echo "waiting for db"; \
		sleep 1; \
	done
	docker compose run --build --rm -e RUN_INTEGRATION=1 app go test -count=1 -run 'TestApply_' ./internal/repository


# Все проверки разом — зеркало pre-commit хука
check:
	@gofmt -l . | grep . && echo "не отформатированы, запустите: make fmt" && exit 1 || true
	golangci-lint run ./...
	go build ./...
	go test -count=1 ./...

# Обновление go.mod / go.sum
tidy:
	go mod tidy
