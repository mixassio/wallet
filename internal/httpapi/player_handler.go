package httpapi

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"github.com/mixassio/wallet/internal/domain"
)

type balanceService interface {
	GetBalance(ctx context.Context, id uuid.UUID) (decimal.Decimal, error)
}

type playerHandler struct {
	svc balanceService
}

type balanceResponse struct {
	Balance decimal.Decimal
}

// MarshalJSON сериализует баланс как JSON-число с двумя знаками (например, 100.00).
func (b balanceResponse) MarshalJSON() ([]byte, error) {
	return []byte(`{"balance":` + b.Balance.StringFixed(2) + `}`), nil
}

func (h *playerHandler) getBalance(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, CodeValidationError, "invalid player id")
		return
	}

	balance, err := h.svc.GetBalance(r.Context(), id)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrNotFound):
			writeError(w, http.StatusNotFound, CodeNotFound, "player not found")
		case errors.Is(err, domain.ErrValidation):
			writeError(w, http.StatusBadRequest, CodeValidationError, err.Error())
		default:
			slog.Error("get balance", "error", err)
			writeError(w, http.StatusInternalServerError, CodeInternal, "internal error")
		}
		return
	}

	writeJSON(w, http.StatusOK, balanceResponse{Balance: balance})
}
