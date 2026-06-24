package repository

import (
	"context"
	"errors"
	"os"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"

	"github.com/mixassio/wallet/internal/domain"
)

func integrationDSN(t *testing.T) string {
	t.Helper()

	if os.Getenv("RUN_INTEGRATION") != "1" {
		t.Skip("RUN_INTEGRATION=1 not set, skipping integration test")
	}

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Fatal("DATABASE_URL not set")
	}

	return dsn
}

// TestApply_ConcurrentIdempotency запускает N параллельных одинаковых withdraw
// с одним transactionId и проверяет, что операция применилась ровно один раз.
//
// Требует живую БД: RUN_INTEGRATION=1 и DATABASE_URL должны быть заданы,
// миграции 001 и 002 — применены.
func TestApply_ConcurrentIdempotency(t *testing.T) {
	dsn := integrationDSN(t)

	ctx := context.Background()

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("connect db: %v", err)
	}
	// LIFO: pool.Close вызывается последним (зарегистрирован первым).
	t.Cleanup(pool.Close)

	playerID := uuid.New()
	txID := uuid.New()

	initial := decimal.RequireFromString("100.00")
	withdraw := decimal.RequireFromString("10.00")
	expected := initial.Sub(withdraw) // 90.00

	_, err = pool.Exec(ctx,
		`INSERT INTO players(id, currency, balance) VALUES($1, $2, $3::numeric)`,
		playerID, "USD", initial.String(),
	)
	if err != nil {
		t.Fatalf("insert player: %v", err)
	}
	// DB-очистка вызывается раньше pool.Close (зарегистрирована позже = LIFO-первая).
	t.Cleanup(func() {
		cleanCtx := context.Background()
		_, _ = pool.Exec(cleanCtx, `DELETE FROM transactions WHERE player_id = $1`, playerID)
		_, _ = pool.Exec(cleanCtx, `DELETE FROM players WHERE id = $1`, playerID)
	})

	repo := NewTransactionRepository(pool)

	const N = 200
	balances := make([]decimal.Decimal, N)
	errs := make([]error, N)

	// Канал закрывается одним вызовом, чтобы все горутины стартовали одновременно.
	ready := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(N)
	for i := range N {
		go func() {
			defer wg.Done()
			<-ready
			balances[i], errs[i] = repo.Apply(ctx, txID, playerID, domain.TxTypeWithdraw, withdraw, "USD")
		}()
	}
	close(ready)
	wg.Wait()

	// Все горутины должны вернуть nil и одинаковый баланс (90.00).
	for i, e := range errs {
		if e != nil {
			t.Errorf("goroutine %d: unexpected error: %v", i, e)
		}
	}
	for i, bal := range balances {
		if !bal.Equal(expected) {
			t.Errorf("goroutine %d: want balance %s, got %s", i, expected, bal)
		}
	}

	// Проверяем финальное состояние в БД.
	var finalStr string
	if err = pool.QueryRow(ctx,
		`SELECT balance::text FROM players WHERE id = $1`, playerID,
	).Scan(&finalStr); err != nil {
		t.Fatalf("read final balance: %v", err)
	}
	final, _ := decimal.NewFromString(finalStr)
	if !final.Equal(expected) {
		t.Errorf("DB balance: want %s, got %s", expected, final)
	}

	var count int
	if err = pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM transactions WHERE id = $1`, txID,
	).Scan(&count); err != nil {
		t.Fatalf("count transactions: %v", err)
	}
	if count != 1 {
		t.Errorf("expected exactly 1 transaction row, got %d", count)
	}
}

func TestApply_DuplicateTransactionIDRejectsMismatchedPayload(t *testing.T) {
	dsn := integrationDSN(t)

	ctx := context.Background()

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("connect db: %v", err)
	}
	t.Cleanup(pool.Close)

	playerID := uuid.New()
	txID := uuid.New()

	initial := decimal.RequireFromString("100.00")
	withdraw := decimal.RequireFromString("10.00")
	expected := decimal.RequireFromString("90.00")

	_, err = pool.Exec(ctx,
		`INSERT INTO players(id, currency, balance) VALUES($1, $2, $3::numeric)`,
		playerID, "USD", initial.String(),
	)
	if err != nil {
		t.Fatalf("insert player: %v", err)
	}
	t.Cleanup(func() {
		cleanCtx := context.Background()
		_, _ = pool.Exec(cleanCtx, `DELETE FROM transactions WHERE player_id = $1`, playerID)
		_, _ = pool.Exec(cleanCtx, `DELETE FROM players WHERE id = $1`, playerID)
	})

	repo := NewTransactionRepository(pool)

	got, err := repo.Apply(ctx, txID, playerID, domain.TxTypeWithdraw, withdraw, "USD")
	if err != nil {
		t.Fatalf("first apply: %v", err)
	}
	if !got.Equal(expected) {
		t.Fatalf("first apply balance: want %s, got %s", expected, got)
	}

	got, err = repo.Apply(ctx, txID, playerID, domain.TxTypeDeposit, decimal.RequireFromString("50.00"), "USD")
	if !errors.Is(err, domain.ErrTransactionFailed) {
		t.Fatalf("duplicate apply: want %v, got balance=%s err=%v", domain.ErrTransactionFailed, got, err)
	}

	var finalStr string
	if err = pool.QueryRow(ctx,
		`SELECT balance::text FROM players WHERE id = $1`,
		playerID,
	).Scan(&finalStr); err != nil {
		t.Fatalf("read final balance: %v", err)
	}
	final, err := decimal.NewFromString(finalStr)
	if err != nil {
		t.Fatalf("parse final balance: %v", err)
	}
	if !final.Equal(expected) {
		t.Fatalf("final balance: want %s, got %s", expected, final)
	}

	var (
		count     int
		txType    string
		amountStr string
	)
	if err = pool.QueryRow(ctx,
		`SELECT COUNT(*), MAX(type), MAX(amount::text) FROM transactions WHERE id = $1`,
		txID,
	).Scan(&count, &txType, &amountStr); err != nil {
		t.Fatalf("read transaction: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected exactly 1 transaction row, got %d", count)
	}
	if txType != string(domain.TxTypeWithdraw) || amountStr != "10.00" {
		t.Fatalf("stored transaction changed: type=%s amount=%s", txType, amountStr)
	}
}
