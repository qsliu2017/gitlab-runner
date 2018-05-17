package common

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestVariablesJSON(t *testing.T) {
	var x JobVariable
	data := []byte(`{"key": "FOO", "value": "bar", "public": true, "internal": true, "file": true, "raw": true}`)

	err := json.Unmarshal(data, &x)
	assert.NoError(t, err)
	assert.Equal(t, "FOO", x.Key)
	assert.Equal(t, "bar", x.Value)
	assert.Equal(t, true, x.Public)
	assert.Equal(t, false, x.Internal) // cannot be set from the network
	assert.Equal(t, true, x.File)
	assert.Equal(t, true, x.Raw)
}

func TestVariableString(t *testing.T) {
	v := JobVariable{Key: "key", Value: "value", Public: false, Internal: false, File: false, Raw: false}
	assert.Equal(t, "key=value", v.String())
}

func TestPublicAndInternalVariables(t *testing.T) {
	v1 := JobVariable{Key: "key", Value: "value", Public: false, Internal: false, File: false, Raw: false}
	v2 := JobVariable{Key: "public", Value: "value", Public: true, Internal: false, File: false, Raw: false}
	v3 := JobVariable{Key: "private", Value: "value", Public: false, Internal: true, File: false, Raw: false}
	all := JobVariables{v1, v2, v3}
	public := all.PublicOrInternal()
	assert.NotContains(t, public, v1)
	assert.Contains(t, public, v2)
	assert.Contains(t, public, v3)
}

func TestListVariables(t *testing.T) {
	v := JobVariables{{Key: "key", Value: "value", Public: false, Internal: false, File: false, Raw: false}}
	assert.Equal(t, []string{"key=value"}, v.StringList())
}

func TestGetVariable(t *testing.T) {
	v1 := JobVariable{Key: "key", Value: "key_value", Public: false, Internal: false, File: false, Raw: false}
	v2 := JobVariable{Key: "public", Value: "public_value", Public: true, Internal: false, File: false, Raw: false}
	v3 := JobVariable{Key: "private", Value: "private_value", Public: false, Internal: false, File: false, Raw: false}
	all := JobVariables{v1, v2, v3}

	assert.Equal(t, "public_value", all.Get("public"))
	assert.Empty(t, all.Get("other"))
}

func TestParseVariable(t *testing.T) {
	v, err := ParseVariable("key=value=value2")
	assert.NoError(t, err)
	assert.Equal(t, JobVariable{Key: "key", Value: "value=value2", Public: false, Internal: false, File: false, Raw: false}, v)
}

func TestInvalidParseVariable(t *testing.T) {
	_, err := ParseVariable("some_other_key")
	assert.Error(t, err)
}

func TestVariablesExpansion(t *testing.T) {
	all := JobVariables{
		{Key: "key", Value: "value_of_$public", Public: false, Internal: false, File: false, Raw: false},
		{Key: "public", Value: "some_value", Public: true, Internal: false, File: false, Raw: false},
		{Key: "private", Value: "value_of_${public}", Public: false, Internal: false, File: false, Raw: false},
		{Key: "public", Value: "value_of_$undefined", Public: true, Internal: false, File: false, Raw: false},
	}

	expanded := all.Expand()
	assert.Len(t, expanded, 4)
	assert.Equal(t, "value_of_value_of_$undefined", expanded.Get("key"))
	assert.Equal(t, "value_of_", expanded.Get("public"))
	assert.Equal(t, "value_of_value_of_$undefined", expanded.Get("private"))
	assert.Equal(t, "value_of_ value_of_value_of_$undefined", expanded.ExpandValue("${public} ${private}"))
}

func TestSpecialVariablesExpansion(t *testing.T) {
	all := JobVariables{
		{Key: "key", Value: "$$", Public: false, Internal: false, File: false, Raw: false},
		{Key: "key2", Value: "$/dsa", Public: true, Internal: false, File: false, Raw: false},
		{Key: "key3", Value: "aa$@bb", Public: false, Internal: false, File: false, Raw: false},
		{Key: "key4", Value: "aa${@}bb", Public: false, Internal: false, File: false, Raw: false},
	}

	expanded := all.Expand()
	assert.Len(t, expanded, 4)
	assert.Equal(t, "$", expanded.Get("key"))
	assert.Equal(t, "/dsa", expanded.Get("key2"))
	assert.Equal(t, "aabb", expanded.Get("key3"))
	assert.Equal(t, "aabb", expanded.Get("key4"))
}

func TestRawVariableExpansion(t *testing.T) {
	tests := []bool{true, false}

	for _, raw := range tests {
		t.Run(fmt.Sprintf("raw-%v", raw), func(t *testing.T) {
			all := JobVariables{
				{Key: "base", Value: "base_value", Public: true, Internal: false, File: false, Raw: false},
				{Key: "related", Value: "value_of_${base}", Public: true, Internal: false, File: false, Raw: raw},
			}
			expanded := all.Expand()
			if raw {
				assert.Equal(t, "value_of_${base}", expanded.Get("related"))
			} else {
				assert.Equal(t, "value_of_base_value", expanded.Get("related"))
			}
		})
	}
}
