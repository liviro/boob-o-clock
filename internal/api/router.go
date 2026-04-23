package api

import "net/http"

func NewRouter(h *Handler) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/session/current", h.GetCurrentSession)
	mux.HandleFunc("POST /api/session/start", h.StartSession)
	mux.HandleFunc("POST /api/session/event", h.PostEvent)
	mux.HandleFunc("POST /api/session/undo", h.PostUndo)
	mux.HandleFunc("GET /api/cycles", h.GetCycles)
	mux.HandleFunc("GET /api/cycles/{id}", h.GetCycleDetail)
	mux.HandleFunc("GET /api/export/csv", h.ExportCSV)

	return mux
}
