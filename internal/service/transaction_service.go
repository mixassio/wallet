package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"github.com/mixassio/wallet/internal/domain"
)

type transactionRepo interface {
	GetPlayerCurrency(ctx context.Context, playerID uuid.UUID) (string, error)
	Apply(ctx context.Context, txID, playerID uuid.UUID, txType domain.TransactionType, amount decimal.Decimal, currency string) (decimal.Decimal, error)
	Cancel(ctx context.Context, txID uuid.UUID) (decimal.Decimal, error)
}

type TransactionService struct {
	repo transactionRepo
}

func NewTransactionService(repo transactionRepo) *TransactionService {
	return &TransactionService{repo: repo}
}

func (s *TransactionService) Apply(
	ctx context.Context,
	txID uuid.UUID,
	playerID uuid.UUID,
	txType domain.TransactionType,
	amount decimal.Decimal,
	currency string,
) (decimal.Decimal, error) {
	if txType != domain.TxTypeDeposit && txType != domain.TxTypeWithdraw {
		return decimal.Decimal{}, fmt.Errorf("unknown transaction type %q: %w", txType, domain.ErrValidation)
	}
	if amount.IsNegative() {
		return decimal.Decimal{}, fmt.Errorf("amount must be non-negative: %w", domain.ErrValidation)
	}
	if len(currency) != 3 {
		return decimal.Decimal{}, fmt.Errorf("currency must be 3 characters: %w", domain.ErrValidation)
	}

	playerCurrency, err := s.repo.GetPlayerCurrency(ctx, playerID)
	if err != nil {
		return decimal.Decimal{}, err
	}
	if !strings.EqualFold(playerCurrency, currency) {
		return decimal.Decimal{}, fmt.Errorf("currency mismatch: player=%s, transaction=%s: %w", playerCurrency, currency, domain.ErrValidation)
	}

	return s.repo.Apply(ctx, txID, playerID, txType, amount, currency)
}

func (s *TransactionService) Cancel(ctx context.Context, txID uuid.UUID) (decimal.Decimal, error) {
	return s.repo.Cancel(ctx, txID)
}
