package service

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"github.com/mixassio/wallet/internal/domain"
)

type fakeTxRepo struct {
	balance        decimal.Decimal
	err            error
	playerCurrency string
	currencyErr    error
}

func (f fakeTxRepo) GetPlayerCurrency(_ context.Context, _ uuid.UUID) (string, error) {
	return f.playerCurrency, f.currencyErr
}

func (f fakeTxRepo) Apply(_ context.Context, _ uuid.UUID, _ uuid.UUID, _ domain.TransactionType, _ decimal.Decimal, _ string) (decimal.Decimal, error) {
	return f.balance, f.err
}

func (f fakeTxRepo) Cancel(_ context.Context, _ uuid.UUID) (decimal.Decimal, error) {
	return f.balance, f.err
}

func TestApplyTransaction(t *testing.T) {
	txID := uuid.New()
	playerID := uuid.New()

	tests := []struct {
		name     string
		repo     fakeTxRepo
		txType   domain.TransactionType
		amount   string
		currency string
		wantBal  string
		wantErr  error
	}{
		{
			name:    "deposit success",
			repo:    fakeTxRepo{balance: decimal.RequireFromString("110.00"), playerCurrency: "USD"},
			txType:  domain.TxTypeDeposit,
			amount:  "10.00",
			wantBal: "110.00",
		},
		{
			name:    "withdraw success",
			repo:    fakeTxRepo{balance: decimal.RequireFromString("90.00"), playerCurrency: "USD"},
			txType:  domain.TxTypeWithdraw,
			amount:  "10.00",
			wantBal: "90.00",
		},
		{
			name:    "zero amount is allowed",
			repo:    fakeTxRepo{balance: decimal.RequireFromString("100.00"), playerCurrency: "USD"},
			txType:  domain.TxTypeDeposit,
			amount:  "0",
			wantBal: "100.00",
		},
		{
			name:    "insufficient funds",
			repo:    fakeTxRepo{err: domain.ErrInsufficientFunds, playerCurrency: "USD"},
			txType:  domain.TxTypeWithdraw,
			amount:  "200.00",
			wantErr: domain.ErrInsufficientFunds,
		},
		{
			name:    "player not found",
			repo:    fakeTxRepo{currencyErr: domain.ErrNotFound},
			txType:  domain.TxTypeDeposit,
			amount:  "10.00",
			wantErr: domain.ErrNotFound,
		},
		{
			name:     "currency mismatch",
			repo:     fakeTxRepo{playerCurrency: "EUR"},
			txType:   domain.TxTypeDeposit,
			amount:   "10.00",
			currency: "USD",
			wantErr:  domain.ErrValidation,
		},
		{
			name:    "invalid type",
			txType:  "transfer",
			amount:  "10.00",
			wantErr: domain.ErrValidation,
		},
		{
			name:    "negative amount",
			txType:  domain.TxTypeDeposit,
			amount:  "-5.00",
			wantErr: domain.ErrValidation,
		},
		{
			name:     "invalid currency length",
			txType:   domain.TxTypeDeposit,
			amount:   "10.00",
			currency: "US",
			wantErr:  domain.ErrValidation,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			currency := tt.currency
			if currency == "" {
				currency = "USD"
			}
			svc := NewTransactionService(tt.repo)
			got, err := svc.Apply(context.Background(), txID, playerID, tt.txType, decimal.RequireFromString(tt.amount), currency)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("expected error %v, got %v", tt.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.StringFixed(2) != tt.wantBal {
				t.Fatalf("expected balance %s, got %s", tt.wantBal, got.StringFixed(2))
			}
		})
	}
}

func TestCancelTransaction(t *testing.T) {
	txID := uuid.New()

	tests := []struct {
		name    string
		repo    fakeTxRepo
		wantBal string
		wantErr error
	}{
		{
			name:    "cancel withdraw success",
			repo:    fakeTxRepo{balance: decimal.RequireFromString("110.00")},
			wantBal: "110.00",
		},
		{
			name:    "deposit cancel error",
			repo:    fakeTxRepo{err: domain.ErrTransactionFailed},
			wantErr: domain.ErrTransactionFailed,
		},
		{
			name:    "idempotent cancel returns balance",
			repo:    fakeTxRepo{balance: decimal.RequireFromString("100.00")},
			wantBal: "100.00",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := NewTransactionService(tt.repo)
			got, err := svc.Cancel(context.Background(), txID)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("expected error %v, got %v", tt.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.StringFixed(2) != tt.wantBal {
				t.Fatalf("expected balance %s, got %s", tt.wantBal, got.StringFixed(2))
			}
		})
	}
}
