package api

import "net/http"

func NewRouter(h *Handler) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/session/current", h.GetCurrentSession)
	mux.HandleFunc("POST /api/session/event", h.PostEvent)
	mux.HandleFunc("POST /api/session/undo", h.PostUndo)
	mux.HandleFunc("GET /api/nights/{id}", h.GetNightDetail)
	mux.HandleFunc("GET /api/nights", h.GetNights)
	mux.HandleFunc("GET /api/trends", h.GetTrends)
	mux.HandleFunc("GET /api/ferber/defaults", h.GetFerberDefaults)
	mux.HandleFunc("GET /api/export/csv", h.ExportCSV)

	return mux
}
