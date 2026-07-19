package server

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/jpequegn/agent-behavior-control-plane/internal/emergency"
)

type Server struct {
	controls *emergency.Manager
	logger   *slog.Logger
}

func (s *Server) WithControls(controls *emergency.Manager) *Server {
	s.controls = controls
	return s
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
	if s.controls != nil {
		mux.HandleFunc("GET /controls", s.listControls)
		mux.HandleFunc("POST /controls", s.applyControl)
		mux.HandleFunc("DELETE /controls/{id}", s.clearControl)
	}
	return mux
}

func (s *Server) listControls(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, s.controls.Active(time.Now().UTC()))
}

func (s *Server) applyControl(w http.ResponseWriter, r *http.Request) {
	var request emergency.Request
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid control request")
		return
	}
	mutation, err := s.controls.Apply(request, time.Now().UTC())
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, mutation)
}

func (s *Server) clearControl(w http.ResponseWriter, r *http.Request) {
	if err := s.controls.Clear(strings.TrimPrefix(r.URL.Path, "/controls/"), time.Now().UTC()); err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	s.logger.Debug("health check", "method", r.Method)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{
		"service": "agent-behavior-control-plane",
		"status":  "ok",
	})
}
