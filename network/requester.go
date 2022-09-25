package network

//go:generate mockery --inpackage --name requester

import "net/http"

type requester interface {
	Do(*http.Request) (*http.Response, error)
}
