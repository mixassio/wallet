package service

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"github.com/mixassio/wallet/internal/domain"
)

type fakeRepo struct {
	player domain.Player
	err    error
}

func (f fakeRepo) GetByID(_ context.Context, _ uuid.UUID) (domain.Player, error) {
	return f.player, f.err
}

func TestGetBalance(t *testing.T) {
	id := uuid.New()

	tests := []struct {
		name    string
		repo    fakeRepo
		want    string
		wantErr error
	}{
		{
			name: "success",
			repo: fakeRepo{player: domain.Player{
				ID:       id,
				Currency: "USD",
				Balance:  decimal.RequireFromString("100.00"),
			}},
			want: "100.00",
		},
		{
			name:    "not found",
			repo:    fakeRepo{err: domain.ErrNotFound},
			wantErr: domain.ErrNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := NewPlayerService(tt.repo)

			got, err := svc.GetBalance(context.Background(), id)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("expected error %v, got %v", tt.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.StringFixed(2) != tt.want {
				t.Fatalf("expected balance %s, got %s", tt.want, got.StringFixed(2))
			}
		})
	}
}
