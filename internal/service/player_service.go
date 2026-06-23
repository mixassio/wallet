package service

import (
	"context"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"github.com/mixassio/wallet/internal/domain"
)

// Зависимость сервиса. Интерфейс объявлен на стороне потребителя, для подмены фейком в тестах.
type playerRepo interface {
	GetByID(ctx context.Context, id uuid.UUID) (domain.Player, error)
}

// бизнес-логика по игрокам
type PlayerService struct {
	repo playerRepo
}

func NewPlayerService(repo playerRepo) *PlayerService {
	return &PlayerService{repo: repo}
}

func (s *PlayerService) GetBalance(ctx context.Context, id uuid.UUID) (decimal.Decimal, error) {
	p, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return decimal.Decimal{}, err
	}
	return p.Balance, nil
}
