package debug

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
)

func SendJSON(w http.ResponseWriter, statusCode int, v interface{}) {
	w.Header().Add("Content-Type", "application/json")

	buff := new(bytes.Buffer)
	encoder := json.NewEncoder(buff)

	err := encoder.Encode(v)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = fmt.Fprintf(w, `{"error":%q}\n`, err)
		return
	}

	w.WriteHeader(statusCode)
	_, _ = fmt.Fprintln(w, buff.String())
}

type ErrorMsg struct {
	Error string `json:"error"`
}
