package testcli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
)

type UnexpectedValueErr struct {
	msg string
}

func (err *UnexpectedValueErr) Error() string {
	return err.msg
}

type Expectation interface {
	Compare(value interface{}) error
	Print() string
}

type keyValueExpectation struct {
	kv map[string]string
}

func (k *keyValueExpectation) Compare(value interface{}) error {
	j, _ := json.Marshal(value)
	var x map[string]interface{}
	_ = json.Unmarshal(j, &x)

	for key, value := range k.kv {
		if x[key] == nil {
			return &UnexpectedValueErr{msg: fmt.Sprintf("Request value for %s is nil", key)}
		}

		switch x[key].(type) {
		case string:
			if value != x[key] {
				return &UnexpectedValueErr{msg: fmt.Sprintf("Value and key mismatch. Got %q but was expecting %q for %q\n", x[key], value, key)}
			}
		case bool:
			if z, _ := strconv.ParseBool(value); z != x[key] {
				return &UnexpectedValueErr{msg: fmt.Sprintf("Bool mismatch: Got %t but was expecting %t for %q\n", x[key], z, key)}
			}

		default:
			return &UnexpectedValueErr{msg: "Don't know what type I just got"}
		}
	}

	return nil
}

func (k *keyValueExpectation) Print() string {
	buf := &bytes.Buffer{}
	_, _ = fmt.Fprintln(buf, "Expectations: ")
	for k, v := range k.kv {
		_, _ = fmt.Fprintf(buf, "\t%q -> %s\n", k, v)
	}

	return buf.String()
}

func NewKeyValueExpectation(kv map[string]string) Expectation {
	return &keyValueExpectation{kv: kv}
}
