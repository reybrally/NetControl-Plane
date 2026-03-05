package http

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

type Server struct {
	Router *chi.Mux
}

func NewServer() *Server {
	r := chi.NewRouter()
	return &Server{Router: r}
}

func (s *Server) Start(addr string) error {
	return http.ListenAndServe(addr, s.Router)
}
