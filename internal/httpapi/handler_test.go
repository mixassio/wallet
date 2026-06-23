package httpapi

import (
	"context"
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

// Поднимает реальный HTTP-сервер с настоящим роутером/middleware и ходит к нему
// http-клиентом, проверяя статус и точное тело ответа.
func TestAPIEndpoints(t *testing.T) {
	srv := httptest.NewServer(NewRouter(newSeededService(), testToken, 5*time.Second))
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
