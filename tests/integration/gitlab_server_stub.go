package testcli

import (
	"encoding/json"
	"fmt"
	"net/http"
)

type GitLabServerStub struct {
	expectations []Expectation
}

type RegisterResponseData struct {
	Token string
	ID    int
}

func NewGitLabServerStub(expectations ...Expectation) *GitLabServerStub {
	return &GitLabServerStub{expectations: expectations}
}

func (g *GitLabServerStub) Start() error {
	fmt.Println("Starting httpd...")
	http.HandleFunc("/api/v4/runners", g.handlerFunc)
	err := http.ListenAndServe(":8080", nil)
	fmt.Println("Stopped httpd...")

	return err
}

func (g *GitLabServerStub) handlerFunc(w http.ResponseWriter, r *http.Request) {
	var jsonData []byte
	jsonData, err := json.Marshal(RegisterResponseData{Token: "badf00d", ID: 42})
	if err != nil {
		fmt.Println("Error marshaling json: ", err)
	}
	w.Header().Set("Content-Type", "application/json")

	for _, expect := range g.expectations {
		if err := expect.Compare(r.Body); err != nil {
			fmt.Println(err)
			w.WriteHeader(http.StatusBadRequest)
		}
	}

	w.WriteHeader(http.StatusCreated)
	fmt.Fprintf(w, string(jsonData))
}
