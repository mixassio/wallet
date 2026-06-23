package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"

	"github.com/mixassio/wallet/internal/domain"
)

type PlayerRepository struct {
	pool *pgxpool.Pool
}

func NewPlayerRepository(pool *pgxpool.Pool) *PlayerRepository {
	return &PlayerRepository{pool: pool}
}

func (r *PlayerRepository) GetByID(ctx context.Context, id uuid.UUID) (domain.Player, error) {
	const query = `SELECT id, currency, balance::text FROM players WHERE id = $1`

	var (
		p          domain.Player
		balanceStr string
	)
	err := r.pool.QueryRow(ctx, query, id).Scan(&p.ID, &p.Currency, &balanceStr)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Player{}, domain.ErrNotFound
		}
		return domain.Player{}, fmt.Errorf("query player: %w", err)
	}

	balance, err := decimal.NewFromString(balanceStr)
	if err != nil {
		return domain.Player{}, fmt.Errorf("parse balance: %w", err)
	}
	p.Balance = balance

	return p, nil
}
