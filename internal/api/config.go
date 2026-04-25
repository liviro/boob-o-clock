package api

import "net/http"

// Config holds runtime feature flags. Zero value = all optional features off,
// the safe production default; opt-in happens via env vars in main.go.
type Config struct {
	FerberEnabled bool
}

type configResponse struct {
	Features configFeatures `json:"features"`
}

type configFeatures struct {
	Ferber bool `json:"ferber"`
}

func (h *Handler) GetConfig(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, configResponse{
		Features: configFeatures{Ferber: h.cfg.FerberEnabled},
	})
}
