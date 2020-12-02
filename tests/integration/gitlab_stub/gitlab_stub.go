package gitlab_stub

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
)

var expectations map[string]string

func Start() {
	expectations = make(map[string]string)
	fmt.Println("Starting httpd...")
	http.HandleFunc("/api/v4/runners", GitlabServer)
	http.ListenAndServe(":8080", nil)
	fmt.Println("Stopped httpd...")
}

type ResponseData struct {
	Token string
	ID    int
}

func GitlabServer(w http.ResponseWriter, r *http.Request) {
	var jsonData []byte
	jsonData, err := json.Marshal(ResponseData{Token: "badf00d", ID: 42})
	if err != nil {
		fmt.Println("Error marshaling json: ", err)
	}
	w.Header().Set("Content-Type", "application/json")

	if expectationsMet(r.Body) {
		w.WriteHeader(http.StatusCreated)
	} else {
		w.WriteHeader(http.StatusBadRequest)
	}
	fmt.Fprintf(w, string(jsonData))
}

type RegistrationRequest struct {
	Token       string
	ID          int
	RunUntagged bool `json:"run_untagged"`
	Locked      bool `json:"locked"`
	Active      bool `json:"active"`
}

func expectationsMet(request io.ReadCloser) bool {
	var r RegistrationRequest
	err := json.NewDecoder(request).Decode(&r)
	if err != nil {
		fmt.Println("Error decoding: ", err)
		return false
	}

	if debug() {
		fmt.Println("R: ", r)

		fmt.Println("Expectations: ")
		for k, v := range expectations {
			fmt.Printf("\t%q -> %s\n", k, v)
		}

	}

	// Need to walk through the expectations struct and compare it against the RegistrationRequest
	j, _ := json.Marshal(r)
	var x map[string]interface{}
	_ = json.Unmarshal(j, &x)
	for key, value := range expectations {
		if x[key] == nil {
			if debug() {
				fmt.Println("Request value for ", key, " is nil")
				// this means the request didn't contain the value.. which is msessed
			}
			return false
		}

		switch x[key].(type) {
		case string:
			if value != x[key] {
				fmt.Printf("Value and key mismatch. Got %q but was expecting %q for %q\n", x[key], value, key)
				return false
			}
		case bool:
			if z, _ := strconv.ParseBool(value); z != x[key] {
				fmt.Printf("Bool mismatch: Got %t but was expecting %t for %q\n", x[key], z, key)
				return false
			}

		default:
			fmt.Println("Don't know what type I just got")
			return false
		}

	}

	return true
}

func SetExpectation(key, value string) {
	expectations[key] = value

	if debug() {
		fmt.Println("Expectations: ")
		for k, v := range expectations {
			fmt.Printf("\t%q -> %s\n", k, v)
		}
	}
}

func debug() bool {
	return os.Getenv("DEBUGTESTS") != ""
}

func ClearExpectations() {
	expectations = make(map[string]string)
}
