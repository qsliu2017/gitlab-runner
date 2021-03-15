package debug

import (
	"net/http"

	"github.com/gorilla/mux"
)

type Server struct {
	router *mux.Router
}

func NewServer(upstream *http.ServeMux) *Server {
	router := mux.NewRouter().
		PathPrefix("/debug").
		Subrouter()

	s := &Server{
		router: router,
	}

	upstream.Handle("/debug/", s)

	return s
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.router.ServeHTTP(w, r)
}
