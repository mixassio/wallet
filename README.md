# wallet

Минимальный backend-сервис «кошелёк» на Go (chi) с чистым разделением слоёв,
PostgreSQL, единым форматом ошибок, авторизацией по Bearer-токену и graceful shutdown.

## Быстрый старт

```bash
cp .env.example .env # при необходимости поменяйте AUTH_TOKEN
make up # поднимет PostgreSQL и сервис на :3000
```

При первой инициализации БД автоматически применяется `migrations/001_init.sql`:
создаётся таблица `players` и создается два игрока

## API

Все эндпоинты защищены — требуется заголовок `Authorization: Bearer <AUTH_TOKEN>`.

### `GET /ping` — health-check

```bash
curl -s -H "Authorization: Bearer super-secret-token" localhost:3000/ping
# {"pong":42}
```

### `GET /players/{id}/balance` — баланс игрока

```bash
curl -s -H "Authorization: Bearer super-secret-token" \
  localhost:3000/players/11111111-1111-1111-1111-111111111111/balance
# {"balance":100.00}
```

## Конфигурация (env)

AUTH_TOKEN=Bearer-токен
DATABASE_URL=DSN
APP_PORT=APP_PORT
POSTGRES_USER=wallet
POSTGRES_PASSWORD=wallet
POSTGRES_DB=wallet

## Тесты

```bash
make test
```

## Полезные команды

```bash
make up     # сборка и запуск
make logs   # логи приложения
make down   # остановка с удалением данных БД
make tidy   # обновить go.mod/go.sum в контейнере
```
