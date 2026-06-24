package httpapi

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func NewRouter(svc balanceService, txSvc transactionService, authToken string, requestTimeout time.Duration) http.Handler {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(requestLogger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(requestTimeout))
	r.Use(authMiddleware(authToken)) // Все эндпоинты защищены Bearer

	ph := &playerHandler{svc: svc}
	th := &transactionHandler{svc: txSvc}

	r.Get("/ping", handlePing)
	r.Get("/players/{id}/balance", ph.getBalance)
	r.Post("/transactions", th.postTransaction)
	r.Delete("/transactions/{transactionId}", th.deleteTransaction)

	return r
}
