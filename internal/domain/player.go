package domain

import (
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

type Player struct {
	ID       uuid.UUID
	Currency string
	Balance  decimal.Decimal
}
