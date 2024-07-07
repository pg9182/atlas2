package atlas

import (
	"fmt"
	"net/http"

	"github.com/r2northstar/atlas/v2/db/pdatadb"
	"github.com/r2northstar/atlas/v2/db/sessiondb"
)

type Config struct {
	// Mux, if provided, specifies the [net/http.ServeMux] to add handlers to.
	Mux *http.ServeMux

	// PdataStorage stores player data.
	PdataStorage *pdatadb.DB

	// SessionStorage stores authentication information.
	SessionStorage *sessiondb.DB
}

type Handler struct {
	cfg Config
}

func New(cfg Config) (*Handler, error) {
	h := &Handler{cfg: cfg}

	if h.cfg.Mux == nil {
		h.cfg.Mux = http.NewServeMux()
	}
	if h.cfg.PdataStorage == nil {
		return nil, fmt.Errorf("pdata storage is required")
	}
	if h.cfg.SessionStorage == nil {
		return nil, fmt.Errorf("session storage is required")
	}

	if err := h.init(); err != nil {
		return nil, fmt.Errorf("init: %w", err)
	}

	return h, nil
}

func (h *Handler) init() error {
	if err := h.initAuth(); err != nil {
		return fmt.Errorf("auth: %w", err)
	}
	if err := h.initPdata(); err != nil {
		return fmt.Errorf("pdata: %w", err)
	}
	if err := h.initServer(); err != nil {
		return fmt.Errorf("server: %w", err)
	}
	if err := h.initMisc(); err != nil {
		return fmt.Errorf("misc: %w", err)
	}
	return nil
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.cfg.Mux.ServeHTTP(w, r)
}
