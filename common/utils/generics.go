package utils

import "encoding/json"

// Keys returns the keys of the map m.
// The keys will be an indeterminate order.
func Keys[M ~map[K]V, K comparable, V any](m M) []K {
	r := make([]K, 0, len(m))
	for k := range m {
		r = append(r, k)
	}
	return r
}

func ConvertObjectTo[T any](obj any) (T, error) {
	b, err := json.Marshal(obj)
	if err != nil {
		return *new(T), err
	}

	var res T
	if err := json.Unmarshal(b, &res); err != nil {
		return *new(T), err
	}

	return res, nil
}
