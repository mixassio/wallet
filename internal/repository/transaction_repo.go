package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"

	"github.com/mixassio/wallet/internal/domain"
)

type TransactionRepository struct {
	pool *pgxpool.Pool
}

func NewTransactionRepository(pool *pgxpool.Pool) *TransactionRepository {
	return &TransactionRepository{pool: pool}
}

// Apply атомарно применяет транзакцию к балансу игрока.
// Идемпотентна: повторный вызов с тем же txID не изменяет баланс.
func (r *TransactionRepository) Apply(
	ctx context.Context,
	txID uuid.UUID,
	playerID uuid.UUID,
	txType domain.TransactionType,
	amount decimal.Decimal,
	currency string,
) (decimal.Decimal, error) {
	pgTx, err := r.pool.Begin(ctx)
	if err != nil {
		return decimal.Decimal{}, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = pgTx.Rollback(ctx) }()

	// Шаг 1: блокируем строку игрока, читаем текущий баланс
	var balanceStr string
	err = pgTx.QueryRow(ctx,
		`SELECT balance::text FROM players WHERE id = $1 FOR UPDATE`,
		playerID,
	).Scan(&balanceStr)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return decimal.Decimal{}, domain.ErrNotFound
		}
		return decimal.Decimal{}, fmt.Errorf("lock player: %w", err)
	}

	balance, err := decimal.NewFromString(balanceStr)
	if err != nil {
		return decimal.Decimal{}, fmt.Errorf("parse balance: %w", err)
	}

	// Шаг 2: пытаемся вставить транзакцию, при конфликте ничего не делаем
	tag, err := pgTx.Exec(ctx,
		`INSERT INTO transactions(id, player_id, type, amount, currency, status)
		 VALUES($1, $2, $3, $4::numeric, $5, 'applied')
		 ON CONFLICT(id) DO NOTHING`,
		txID, playerID, string(txType), amount.String(), currency,
	)
	if err != nil {
		return decimal.Decimal{}, fmt.Errorf("insert transaction: %w", err)
	}

	// Шаг 3: txID уже существует то идемпотентный путь
	if tag.RowsAffected() == 0 {
		var (
			storedPlayerID  uuid.UUID
			storedType      string
			storedAmountStr string
			storedCurrency  string
			status          string
		)
		err = pgTx.QueryRow(ctx,
			`SELECT player_id, type, amount::text, currency, status FROM transactions WHERE id = $1`,
			txID,
		).Scan(&storedPlayerID, &storedType, &storedAmountStr, &storedCurrency, &status)
		if err != nil {
			return decimal.Decimal{}, fmt.Errorf("query transaction: %w", err)
		}

		if status == "cancelled" {
			// Отменённый txID нельзя переиспользовать
			return decimal.Decimal{}, domain.ErrTransactionFailed
		}

		storedAmount, err := decimal.NewFromString(storedAmountStr)
		if err != nil {
			return decimal.Decimal{}, fmt.Errorf("parse stored amount: %w", err)
		}
		if storedPlayerID != playerID ||
			storedType != string(txType) ||
			!storedAmount.Equal(amount) ||
			!strings.EqualFold(strings.TrimSpace(storedCurrency), strings.TrimSpace(currency)) {
			return decimal.Decimal{}, domain.ErrTransactionFailed
		}

		// status == 'applied' and payload matches: дублирующий запрос то возвращаем текущий баланс.
		if err = pgTx.Commit(ctx); err != nil {
			return decimal.Decimal{}, fmt.Errorf("commit: %w", err)
		}
		return balance, nil
	}

	// Шаг 4: новая транзакция — проверяем средства и обновляем баланс
	if txType == domain.TxTypeWithdraw && balance.LessThan(amount) {
		return decimal.Decimal{}, domain.ErrInsufficientFunds
	}

	delta := amount
	if txType == domain.TxTypeWithdraw {
		delta = amount.Neg()
	}

	var newBalanceStr string
	if err = pgTx.QueryRow(ctx,
		`UPDATE players SET balance = balance + $2::numeric WHERE id = $1 RETURNING balance::text`,
		playerID, delta.String(),
	).Scan(&newBalanceStr); err != nil {
		return decimal.Decimal{}, fmt.Errorf("update balance: %w", err)
	}

	newBalance, err := decimal.NewFromString(newBalanceStr)
	if err != nil {
		return decimal.Decimal{}, fmt.Errorf("parse new balance: %w", err)
	}

	if err = pgTx.Commit(ctx); err != nil {
		return decimal.Decimal{}, fmt.Errorf("commit: %w", err)
	}
	return newBalance, nil
}

// GetPlayerCurrency возвращает валюту игрока для проверки на уровне сервиса
func (r *TransactionRepository) GetPlayerCurrency(ctx context.Context, playerID uuid.UUID) (string, error) {
	var currency string
	err := r.pool.QueryRow(ctx, `SELECT currency FROM players WHERE id = $1`, playerID).Scan(&currency)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", domain.ErrNotFound
		}
		return "", fmt.Errorf("query player currency: %w", err)
	}
	return currency, nil
}

// Cancel атомарно отменяет withdraw-транзакцию, возвращая средства игроку.
// Идемпотентна: повторная отмена возвращает текущий баланс без изменений.
func (r *TransactionRepository) Cancel(
	ctx context.Context,
	txID uuid.UUID,
) (decimal.Decimal, error) {
	pgTx, err := r.pool.Begin(ctx)
	if err != nil {
		return decimal.Decimal{}, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = pgTx.Rollback(ctx) }()

	// Шаг 1: читаем транзакцию без блокировки.
	var (
		playerID  uuid.UUID
		txType    string
		amountStr string
		status    string
	)
	err = pgTx.QueryRow(ctx,
		`SELECT player_id, type, amount::text, status FROM transactions WHERE id = $1`,
		txID,
	).Scan(&playerID, &txType, &amountStr, &status)

	if errors.Is(err, pgx.ErrNoRows) {
		// Транзакция не существует, возвращаем нулевой баланс как успех. (Мы не можем вернуть текущий баланс - кого???)
		if err = pgTx.Commit(ctx); err != nil {
			return decimal.Decimal{}, fmt.Errorf("commit: %w", err)
		}
		return decimal.Zero, nil
	}
	if err != nil {
		return decimal.Decimal{}, fmt.Errorf("query transaction: %w", err)
	}

	if txType == string(domain.TxTypeDeposit) {
		return decimal.Decimal{}, domain.ErrTransactionFailed
	}

	amount, err := decimal.NewFromString(amountStr)
	if err != nil {
		return decimal.Decimal{}, fmt.Errorf("parse amount: %w", err)
	}

	// Шаг 2: блокируем строку игрока, читаем текущий баланс
	var balanceStr string
	if err = pgTx.QueryRow(ctx,
		`SELECT balance::text FROM players WHERE id = $1 FOR UPDATE`,
		playerID,
	).Scan(&balanceStr); err != nil {
		return decimal.Decimal{}, fmt.Errorf("lock player: %w", err)
	}

	balance, err := decimal.NewFromString(balanceStr)
	if err != nil {
		return decimal.Decimal{}, fmt.Errorf("parse balance: %w", err)
	}

	// Шаг 3: CAS-обновление статуса транзакции
	tag, err := pgTx.Exec(ctx,
		`UPDATE transactions SET status = 'cancelled'
		 WHERE id = $1 AND status = 'applied' AND type = 'withdraw'`,
		txID,
	)
	if err != nil {
		return decimal.Decimal{}, fmt.Errorf("cancel transaction: %w", err)
	}

	if tag.RowsAffected() == 0 {
		// Уже отменено параллельным запросом то идемпотентный ответ
		if err = pgTx.Commit(ctx); err != nil {
			return decimal.Decimal{}, fmt.Errorf("commit: %w", err)
		}
		return balance, nil
	}

	// Шаг 4: возвращаем средства игроку
	var newBalanceStr string
	if err = pgTx.QueryRow(ctx,
		`UPDATE players SET balance = balance + $2::numeric WHERE id = $1 RETURNING balance::text`,
		playerID, amount.String(),
	).Scan(&newBalanceStr); err != nil {
		return decimal.Decimal{}, fmt.Errorf("restore balance: %w", err)
	}

	newBalance, err := decimal.NewFromString(newBalanceStr)
	if err != nil {
		return decimal.Decimal{}, fmt.Errorf("parse new balance: %w", err)
	}

	if err = pgTx.Commit(ctx); err != nil {
		return decimal.Decimal{}, fmt.Errorf("commit: %w", err)
	}
	return newBalance, nil
}
