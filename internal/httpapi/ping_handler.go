package httpapi

import "net/http"

type pingResponse struct {
	Pong int `json:"pong"`
}

// health-check
func handlePing(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, pingResponse{Pong: 42})
}
