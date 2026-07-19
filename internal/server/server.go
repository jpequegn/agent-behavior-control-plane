package server

import (
	"encoding/json"
	"log/slog"
	"net/http"
)

type Server struct {
	logger *slog.Logger
}

func New(logger *slog.Logger) *Server {
	if logger == nil {
		logger = slog.Default()
	}
	return &Server{logger: logger}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.health)
	return mux
}

func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	s.logger.Debug("health check", "method", r.Method)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{
		"service": "agent-behavior-control-plane",
		"status":  "ok",
	})
}
