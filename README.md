# wallet

Минимальный backend-сервис «кошелёк» на Go (chi) с PostgreSQL, единым форматом
ошибок, Bearer-авторизацией, таймаутами запросов и graceful shutdown.

## Быстрый старт

```bash
cp .env.example .env # при необходимости поменяйте AUTH_TOKEN
make up # поднимет PostgreSQL и сервис на :3000
```

Сервис доступен на `localhost:3000`. При первой инициализации Postgres применяет
миграции из `migrations/`: создаёт `players`, `transactions` и сидит двух игроков.

## API

Все эндпоинты защищены, требуется заголовок `Authorization: Bearer <AUTH_TOKEN>`.

### `GET /ping`

```bash
curl -s -H "Authorization: Bearer super-secret-token" localhost:3000/ping
# {"pong":42}
```

### `GET /players/{id}/balance`

```bash
curl -s -H "Authorization: Bearer super-secret-token" \
  localhost:3000/players/11111111-1111-1111-1111-111111111111/balance
# {"balance":100.00}
```

### `POST /transactions`

`transactionId` - ключ идемпотентности. Повтор с тем же `transactionId` и тем же
payload не применяет операцию повторно и возвращает текущий баланс. Если payload
отличается, сервис возвращает `TRANSACTION_FAILED`. `amount: 0` допустим.

```bash
curl -s -X POST -H "Authorization: Bearer super-secret-token" \
  -H "Content-Type: application/json" \
  -d '{"transactionId":"aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa","playerId":"11111111-1111-1111-1111-111111111111","type":"withdraw","amount":10.50,"currency":"USD"}' \
  localhost:3000/transactions
# {"balance":89.50}
```

### `DELETE /transactions/{transactionId}`

Отменяет ранее проведённый `withdraw` ровно один раз. `deposit` отменять нельзя.
Повторная отмена не меняет баланс. Если `transactionId` не найден, endpoint
возвращает `{"balance":0.00}`: в URL нет `playerId`, поэтому текущий баланс
конкретного игрока определить нельзя.

```bash
curl -s -X DELETE -H "Authorization: Bearer super-secret-token" \
  localhost:3000/transactions/aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa
# {"balance":100.00}
```

Ошибки возвращаются в едином формате:

```json
{ "error": { "code": "INSUFFICIENT_FUNDS", "message": "insufficient funds" } }
```

## Хранилище и гарантии

Выбран PostgreSQL: он даёт транзакции, row-level locks и ограничения целостности
без дополнительной инфраструктуры. `transactions.id` - primary key, поэтому
уникальность `transactionId` обеспечивается БД. Запись транзакции и изменение
баланса выполняются в одной DB transaction. Баланс игрока блокируется через
`SELECT ... FOR UPDATE`, что защищает от потерянных обновлений при параллельных
запросах. `players.balance` имеет `CHECK (balance >= 0)`, а частые запросы по
истории игрока поддержаны индексом `idx_transactions_player_id`.

## Конфигурация

```env
AUTH_TOKEN=super-secret-token
DATABASE_URL=postgres://wallet:wallet@localhost:5432/wallet?sslmode=disable
APP_PORT=3000
POSTGRES_USER=wallet
POSTGRES_PASSWORD=wallet
POSTGRES_DB=wallet
```

## Тесты и команды

```bash
make test              # unit/API тесты без кэша
make test-integration  # поднимает db и запускает concurrency/idempotency тесты
make check             # gofmt check, lint, build, tests
make logs              # логи приложения
make down              # остановка с удалением данных БД
```

Для интеграционных тестов задайте `DATABASE_URL`; команда сама поднимет сервис
`db` через Docker Compose:

```bash
DATABASE_URL=postgres://wallet:wallet@localhost:5432/wallet?sslmode=disable make test-integration
```
