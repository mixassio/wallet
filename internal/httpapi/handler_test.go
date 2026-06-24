package httpapi

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"github.com/mixassio/wallet/internal/domain"
)

const testToken = "super-secret-token"

// фейковый сервис с теми же игроками, что в migrations/001_init.sql.
type seededService struct {
	players map[uuid.UUID]decimal.Decimal
}

func newSeededService() seededService {
	return seededService{players: map[uuid.UUID]decimal.Decimal{
		uuid.MustParse("11111111-1111-1111-1111-111111111111"): decimal.RequireFromString("100.00"),
		uuid.MustParse("22222222-2222-2222-2222-222222222222"): decimal.RequireFromString("50.50"),
	}}
}

func (s seededService) GetBalance(_ context.Context, id uuid.UUID) (decimal.Decimal, error) {
	balance, ok := s.players[id]
	if !ok {
		return decimal.Decimal{}, domain.ErrNotFound
	}
	return balance, nil
}

type fakeTransactionService struct {
	balance decimal.Decimal
	err     error
}

func (f fakeTransactionService) Apply(_ context.Context, _ uuid.UUID, _ uuid.UUID, _ domain.TransactionType, _ decimal.Decimal, _ string) (decimal.Decimal, error) {
	return f.balance, f.err
}

func (f fakeTransactionService) Cancel(_ context.Context, _ uuid.UUID) (decimal.Decimal, error) {
	return f.balance, f.err
}

// Поднимает реальный HTTP-сервер с настоящим роутером/middleware и ходит к нему
// http-клиентом, проверяя статус и точное тело ответа.
func TestAPIEndpoints(t *testing.T) {
	srv := httptest.NewServer(NewRouter(newSeededService(), fakeTransactionService{}, testToken, 5*time.Second))
	defer srv.Close()

	tests := []struct {
		name       string
		path       string
		token      string
		wantStatus int
		wantBody   string
	}{
		{
			name:       "1. без токена - 401 UNAUTHORIZED",
			path:       "/ping",
			token:      "",
			wantStatus: http.StatusUnauthorized,
			wantBody:   `{"error":{"code":"UNAUTHORIZED","message":"missing or invalid token"}}`,
		},
		{
			name:       "2. health-check - 200 {pong:42}",
			path:       "/ping",
			token:      testToken,
			wantStatus: http.StatusOK,
			wantBody:   `{"pong":42}`,
		},
		{
			name:       "3. баланс игрока 1 - 200 100.00",
			path:       "/players/11111111-1111-1111-1111-111111111111/balance",
			token:      testToken,
			wantStatus: http.StatusOK,
			wantBody:   `{"balance":100.00}`,
		},
		{
			name:       "4. баланс игрока 2 - 200 50.50",
			path:       "/players/22222222-2222-2222-2222-222222222222/balance",
			token:      testToken,
			wantStatus: http.StatusOK,
			wantBody:   `{"balance":50.50}`,
		},
		{
			name:       "5. битый id - 400 VALIDATION_ERROR",
			path:       "/players/not-a-uuid/balance",
			token:      testToken,
			wantStatus: http.StatusBadRequest,
			wantBody:   `{"error":{"code":"VALIDATION_ERROR","message":"invalid player id"}}`,
		},
		{
			name:       "6. несуществующий игрок - 404 NOT_FOUND",
			path:       "/players/00000000-0000-0000-0000-000000000000/balance",
			token:      testToken,
			wantStatus: http.StatusNotFound,
			wantBody:   `{"error":{"code":"NOT_FOUND","message":"player not found"}}`,
		},
		{
			name:       "7. неверный токен - 401 UNAUTHORIZED",
			path:       "/ping",
			token:      "wrong",
			wantStatus: http.StatusUnauthorized,
			wantBody:   `{"error":{"code":"UNAUTHORIZED","message":"missing or invalid token"}}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := http.NewRequest(http.MethodGet, srv.URL+tt.path, nil)
			if err != nil {
				t.Fatalf("build request: %v", err)
			}
			if tt.token != "" {
				req.Header.Set("Authorization", "Bearer "+tt.token)
			}

			resp, err := srv.Client().Do(req)
			if err != nil {
				t.Fatalf("do request: %v", err)
			}
			defer func() { _ = resp.Body.Close() }()

			body, err := io.ReadAll(resp.Body)
			if err != nil {
				t.Fatalf("read body: %v", err)
			}

			if resp.StatusCode != tt.wantStatus {
				t.Fatalf("status: want %d, got %d (body: %s)", tt.wantStatus, resp.StatusCode, body)
			}
			if got := strings.TrimSpace(string(body)); got != tt.wantBody {
				t.Fatalf("body:\n want %s\n got  %s", tt.wantBody, got)
			}
		})
	}
}

func TestTransactionEndpoints(t *testing.T) {
	const (
		validTxID     = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
		validPlayerID = "11111111-1111-1111-1111-111111111111"
	)

	depositBody := `{"transactionId":"` + validTxID + `","playerId":"` + validPlayerID + `","type":"deposit","amount":10.50,"currency":"USD"}`

	tests := []struct {
		name       string
		method     string
		path       string
		body       string
		fakeSvc    fakeTransactionService
		wantStatus int
		wantBody   string
	}{
		{
			name:       "POST deposit success",
			method:     http.MethodPost,
			path:       "/transactions",
			body:       depositBody,
			fakeSvc:    fakeTransactionService{balance: decimal.RequireFromString("110.50")},
			wantStatus: http.StatusOK,
			wantBody:   `{"balance":110.50}`,
		},
		{
			name:       "POST withdraw success",
			method:     http.MethodPost,
			path:       "/transactions",
			body:       `{"transactionId":"` + validTxID + `","playerId":"` + validPlayerID + `","type":"withdraw","amount":10.50,"currency":"USD"}`,
			fakeSvc:    fakeTransactionService{balance: decimal.RequireFromString("89.50")},
			wantStatus: http.StatusOK,
			wantBody:   `{"balance":89.50}`,
		},
		{
			name:       "POST zero amount is allowed",
			method:     http.MethodPost,
			path:       "/transactions",
			body:       `{"transactionId":"` + validTxID + `","playerId":"` + validPlayerID + `","type":"deposit","amount":0,"currency":"USD"}`,
			fakeSvc:    fakeTransactionService{balance: decimal.RequireFromString("100.00")},
			wantStatus: http.StatusOK,
			wantBody:   `{"balance":100.00}`,
		},
		{
			name:       "POST idempotent duplicate returns current balance",
			method:     http.MethodPost,
			path:       "/transactions",
			body:       depositBody,
			fakeSvc:    fakeTransactionService{balance: decimal.RequireFromString("110.50")},
			wantStatus: http.StatusOK,
			wantBody:   `{"balance":110.50}`,
		},
		{
			name:       "POST insufficient funds",
			method:     http.MethodPost,
			path:       "/transactions",
			body:       `{"transactionId":"` + validTxID + `","playerId":"` + validPlayerID + `","type":"withdraw","amount":999.00,"currency":"USD"}`,
			fakeSvc:    fakeTransactionService{err: domain.ErrInsufficientFunds},
			wantStatus: http.StatusUnprocessableEntity,
			wantBody:   `{"error":{"code":"INSUFFICIENT_FUNDS","message":"insufficient funds"}}`,
		},
		{
			name:       "POST validation error from service",
			method:     http.MethodPost,
			path:       "/transactions",
			body:       `{"transactionId":"` + validTxID + `","playerId":"` + validPlayerID + `","type":"transfer","amount":10.50,"currency":"USD"}`,
			fakeSvc:    fakeTransactionService{err: fmt.Errorf("unknown type: %w", domain.ErrValidation)},
			wantStatus: http.StatusBadRequest,
			wantBody:   `{"error":{"code":"VALIDATION_ERROR","message":"unknown type: validation error"}}`,
		},
		{
			name:       "POST transaction failed from service",
			method:     http.MethodPost,
			path:       "/transactions",
			body:       depositBody,
			fakeSvc:    fakeTransactionService{err: domain.ErrTransactionFailed},
			wantStatus: http.StatusUnprocessableEntity,
			wantBody:   `{"error":{"code":"TRANSACTION_FAILED","message":"transaction failed"}}`,
		},
		{
			name:       "POST invalid JSON body",
			method:     http.MethodPost,
			path:       "/transactions",
			body:       `{bad json`,
			wantStatus: http.StatusBadRequest,
			wantBody:   `{"error":{"code":"VALIDATION_ERROR","message":"invalid request body"}}`,
		},
		{
			name:       "POST missing transactionId",
			method:     http.MethodPost,
			path:       "/transactions",
			body:       `{"playerId":"` + validPlayerID + `","type":"deposit","amount":10.50,"currency":"USD"}`,
			wantStatus: http.StatusBadRequest,
			wantBody:   `{"error":{"code":"VALIDATION_ERROR","message":"invalid transactionId"}}`,
		},
		{
			name:       "POST missing playerId",
			method:     http.MethodPost,
			path:       "/transactions",
			body:       `{"transactionId":"` + validTxID + `","type":"deposit","amount":10.50,"currency":"USD"}`,
			wantStatus: http.StatusBadRequest,
			wantBody:   `{"error":{"code":"VALIDATION_ERROR","message":"invalid playerId"}}`,
		},
		{
			name:       "POST missing type",
			method:     http.MethodPost,
			path:       "/transactions",
			body:       `{"transactionId":"` + validTxID + `","playerId":"` + validPlayerID + `","amount":10.50,"currency":"USD"}`,
			wantStatus: http.StatusBadRequest,
			wantBody:   `{"error":{"code":"VALIDATION_ERROR","message":"type is required"}}`,
		},
		{
			name:       "POST missing amount",
			method:     http.MethodPost,
			path:       "/transactions",
			body:       `{"transactionId":"` + validTxID + `","playerId":"` + validPlayerID + `","type":"deposit","currency":"USD"}`,
			wantStatus: http.StatusBadRequest,
			wantBody:   `{"error":{"code":"VALIDATION_ERROR","message":"amount is required"}}`,
		},
		{
			name:       "POST null amount",
			method:     http.MethodPost,
			path:       "/transactions",
			body:       `{"transactionId":"` + validTxID + `","playerId":"` + validPlayerID + `","type":"deposit","amount":null,"currency":"USD"}`,
			wantStatus: http.StatusBadRequest,
			wantBody:   `{"error":{"code":"VALIDATION_ERROR","message":"amount is required"}}`,
		},
		{
			name:       "POST missing currency",
			method:     http.MethodPost,
			path:       "/transactions",
			body:       `{"transactionId":"` + validTxID + `","playerId":"` + validPlayerID + `","type":"deposit","amount":10.50}`,
			wantStatus: http.StatusBadRequest,
			wantBody:   `{"error":{"code":"VALIDATION_ERROR","message":"currency is required"}}`,
		},
		{
			name:       "POST invalid transactionId UUID",
			method:     http.MethodPost,
			path:       "/transactions",
			body:       `{"transactionId":"not-a-uuid","playerId":"` + validPlayerID + `","type":"deposit","amount":10.50,"currency":"USD"}`,
			wantStatus: http.StatusBadRequest,
			wantBody:   `{"error":{"code":"VALIDATION_ERROR","message":"invalid transactionId"}}`,
		},
		{
			name:       "POST invalid playerId UUID",
			method:     http.MethodPost,
			path:       "/transactions",
			body:       `{"transactionId":"` + validTxID + `","playerId":"not-a-uuid","type":"deposit","amount":10.50,"currency":"USD"}`,
			wantStatus: http.StatusBadRequest,
			wantBody:   `{"error":{"code":"VALIDATION_ERROR","message":"invalid playerId"}}`,
		},
		{
			name:       "DELETE cancel withdraw success",
			method:     http.MethodDelete,
			path:       "/transactions/" + validTxID,
			fakeSvc:    fakeTransactionService{balance: decimal.RequireFromString("100.00")},
			wantStatus: http.StatusOK,
			wantBody:   `{"balance":100.00}`,
		},
		{
			name:       "DELETE non-existent transactionId returns zero balance",
			method:     http.MethodDelete,
			path:       "/transactions/" + validTxID,
			fakeSvc:    fakeTransactionService{balance: decimal.Zero},
			wantStatus: http.StatusOK,
			wantBody:   `{"balance":0.00}`,
		},
		{
			name:       "DELETE cancel deposit returns error",
			method:     http.MethodDelete,
			path:       "/transactions/" + validTxID,
			fakeSvc:    fakeTransactionService{err: domain.ErrTransactionFailed},
			wantStatus: http.StatusUnprocessableEntity,
			wantBody:   `{"error":{"code":"TRANSACTION_FAILED","message":"cannot cancel deposit"}}`,
		},
		{
			name:       "DELETE invalid transactionId UUID",
			method:     http.MethodDelete,
			path:       "/transactions/not-a-uuid",
			wantStatus: http.StatusBadRequest,
			wantBody:   `{"error":{"code":"VALIDATION_ERROR","message":"invalid transactionId"}}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(NewRouter(newSeededService(), tt.fakeSvc, testToken, 5*time.Second))
			defer srv.Close()

			var body io.Reader
			if tt.body != "" {
				body = strings.NewReader(tt.body)
			}

			req, err := http.NewRequest(tt.method, srv.URL+tt.path, body)
			if err != nil {
				t.Fatalf("build request: %v", err)
			}
			req.Header.Set("Authorization", "Bearer "+testToken)
			if tt.body != "" {
				req.Header.Set("Content-Type", "application/json")
			}

			resp, err := srv.Client().Do(req)
			if err != nil {
				t.Fatalf("do request: %v", err)
			}
			defer func() { _ = resp.Body.Close() }()

			respBody, err := io.ReadAll(resp.Body)
			if err != nil {
				t.Fatalf("read body: %v", err)
			}

			if resp.StatusCode != tt.wantStatus {
				t.Fatalf("status: want %d, got %d (body: %s)", tt.wantStatus, resp.StatusCode, respBody)
			}
			if got := strings.TrimSpace(string(respBody)); got != tt.wantBody {
				t.Fatalf("body:\n want %s\n got  %s", tt.wantBody, got)
			}
		})
	}
}
