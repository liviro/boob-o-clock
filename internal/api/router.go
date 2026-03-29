package api

import "net/http"

func NewRouter(h *Handler) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/session/current", h.GetCurrentSession)
	mux.HandleFunc("POST /api/session/event", h.PostEvent)
	mux.HandleFunc("POST /api/session/undo", h.PostUndo)
	mux.HandleFunc("GET /api/nights/{id}", h.GetNightDetail)
	mux.HandleFunc("GET /api/nights", h.GetNights)

	return mux
}
