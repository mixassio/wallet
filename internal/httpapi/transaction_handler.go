package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"github.com/mixassio/wallet/internal/domain"
)

type transactionService interface {
	Apply(ctx context.Context, txID, playerID uuid.UUID, txType domain.TransactionType, amount decimal.Decimal, currency string) (decimal.Decimal, error)
	Cancel(ctx context.Context, txID uuid.UUID) (decimal.Decimal, error)
}

type transactionHandler struct {
	svc transactionService
}

type applyRequest struct {
	TransactionID string           `json:"transactionId"`
	PlayerID      string           `json:"playerId"`
	Type          string           `json:"type"`
	Amount        *decimal.Decimal `json:"amount"`
	Currency      string           `json:"currency"`
}

func (h *transactionHandler) postTransaction(w http.ResponseWriter, r *http.Request) {
	var req applyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, CodeValidationError, "invalid request body")
		return
	}

	txID, err := uuid.Parse(req.TransactionID)
	if err != nil {
		writeError(w, http.StatusBadRequest, CodeValidationError, "invalid transactionId")
		return
	}
	playerID, err := uuid.Parse(req.PlayerID)
	if err != nil {
		writeError(w, http.StatusBadRequest, CodeValidationError, "invalid playerId")
		return
	}

	if req.Type == "" {
		writeError(w, http.StatusBadRequest, CodeValidationError, "type is required")
		return
	}
	if req.Amount == nil {
		writeError(w, http.StatusBadRequest, CodeValidationError, "amount is required")
		return
	}
	if req.Currency == "" {
		writeError(w, http.StatusBadRequest, CodeValidationError, "currency is required")
		return
	}

	balance, err := h.svc.Apply(r.Context(), txID, playerID, domain.TransactionType(req.Type), *req.Amount, req.Currency)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrNotFound):
			writeError(w, http.StatusNotFound, CodeNotFound, "player not found")
		case errors.Is(err, domain.ErrValidation):
			writeError(w, http.StatusBadRequest, CodeValidationError, err.Error())
		case errors.Is(err, domain.ErrInsufficientFunds):
			writeError(w, http.StatusUnprocessableEntity, CodeInsufficientFunds, "insufficient funds")
		case errors.Is(err, domain.ErrTransactionFailed):
			writeError(w, http.StatusUnprocessableEntity, CodeTransactionFailed, "transaction failed")
		default:
			slog.Error("apply transaction", "error", err)
			writeError(w, http.StatusInternalServerError, CodeInternal, "internal error")
		}
		return
	}

	writeJSON(w, http.StatusOK, balanceResponse{Balance: balance})
}

func (h *transactionHandler) deleteTransaction(w http.ResponseWriter, r *http.Request) {
	txID, err := uuid.Parse(chi.URLParam(r, "transactionId"))
	if err != nil {
		writeError(w, http.StatusBadRequest, CodeValidationError, "invalid transactionId")
		return
	}

	balance, err := h.svc.Cancel(r.Context(), txID)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrTransactionFailed):
			writeError(w, http.StatusUnprocessableEntity, CodeTransactionFailed, "cannot cancel deposit")
		default:
			slog.Error("cancel transaction", "error", err)
			writeError(w, http.StatusInternalServerError, CodeInternal, "internal error")
		}
		return
	}

	writeJSON(w, http.StatusOK, balanceResponse{Balance: balance})
}
