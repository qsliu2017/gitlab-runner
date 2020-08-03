package trace

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVariablesMasking(t *testing.T) {
	traceMessage := "This is the secret message cont@ining :secret duplicateValues ffixx"
	maskedValues := []string{
		"is",
		"duplicateValue",
		"duplicateValue",
		":secret",
		"cont@ining",
		"fix",
	}

	buffer, err := New()
	require.NoError(t, err)
	defer buffer.Close()

	buffer.SetMasked(maskedValues)

	_, err = buffer.Write([]byte(traceMessage))
	require.NoError(t, err)

	buffer.Finish()

	content, err := buffer.Bytes(0, 1000)
	require.NoError(t, err)

	assert.Equal(t, "Th[MASKED] [MASKED] the secret message [MASKED] [MASKED] [MASKED]s f[MASKED]x", string(content))
}

func TestVariablesMaskingBoundary(t *testing.T) {
	tests := map[string]struct {
		values   []string
		expected string
	}{
		"no escaping at all http://example.org/?test=foobar": {
			expected: "no escaping at all http://example.org/?test=foobar",
		},
		"at the start of the buffer": {
			values:   []string{"at"},
			expected: "[MASKED] the start of the buffer",
		},
		"in the middle of the buffer": {
			values:   []string{"middle"},
			expected: "in the [MASKED] of the buffer",
		},
		"at the end of the buffer": {
			values:   []string{"buffer"},
			expected: "at the end of the [MASKED]",
		},
		"all values are masked": {
			values:   []string{"all", "values", "are", "masked"},
			expected: "[MASKED] [MASKED] [MASKED] [MASKED]",
		},
		"prefixed and suffixed: xfoox ybary ffoo barr ffooo bbarr": {
			values:   []string{"foo", "bar"},
			expected: "prefixed and suffixed: x[MASKED]x y[MASKED]y f[MASKED] [MASKED]r f[MASKED]o b[MASKED]r",
		},
		"prefix|ed, su|ffi|xed |and split|:| xfo|ox y|bary ffo|o ba|rr ffooo b|barr": {
			values:   []string{"foo", "bar"},
			expected: "prefixed, suffixed and split: x[MASKED]x y[MASKED]y f[MASKED] [MASKED]r f[MASKED]o b[MASKED]r",
		},
		"http://example.com/?private_token=deadbeef sensitive URL at the start": {
			expected: "http://example.com/?private_token=[MASKED] sensitive URL at the start",
		},
		"a sensitive URL at the end http://example.com/?authenticity_token=deadbeef": {
			expected: "a sensitive URL at the end http://example.com/?authenticity_token=[MASKED]",
		},
		"a sensitive URL http://example.com/?rss_token=deadbeef in the middle": {
			expected: "a sensitive URL http://example.com/?rss_token=[MASKED] in the middle",
		},
		"a sensitive URL http://example.com/?X-AMZ-sigNATure=deadbeef with mixed case": {
			expected: "a sensitive URL http://example.com/?X-AMZ-sigNATure=[MASKED] with mixed case",
		},
		"a sensitive URL http://example.com/?param=second&x-amz-credential=deadbeef second param": {
			expected: "a sensitive URL http://example.com/?param=second&x-amz-credential=[MASKED] second param",
		},
		"a sensitive URL http://example.com/?rss_token=hide&x-amz-credential=deadbeef both params": {
			expected: "a sensitive URL http://example.com/?rss_token=[MASKED]&x-amz-credential=[MASKED] both params",
		},
		//nolint:lll
		"a long sensitive URL http://example.com/?x-amz-credential=abcdefghijklmnopqrstuvwxyz0123456789abcdefghijklmnopqrstuvwxyz0123456789abcdefghijklmnopqrstuvwxyz0123456789": {
			expected: "a long sensitive URL http://example.com/?x-amz-credential=[MASKED]",
		},
		"sp|lit al|l val|ues ar|e |mask|ed": {
			values:   []string{"split", "all", "values", "are", "masked"},
			expected: "[MASKED] [MASKED] [MASKED] [MASKED] [MASKED]",
		},
		"spl|it sensit|ive UR|L http://example.com/?x-amz-cred|ential=abcdefghij|klmnopqrstuvwxyz01234567": {
			expected: "split sensitive URL http://example.com/?x-amz-credential=[MASKED]",
		},
	}

	for tn, tc := range tests {
		t.Run(tn, func(t *testing.T) {
			func() {
				buffer, err := New()
				require.NoError(t, err)
				defer buffer.Close()

				buffer.SetMasked(tc.values)

				for _, part := range bytes.Split([]byte(tn), []byte{'|'}) {
					_, err = buffer.Write(part)
					require.NoError(t, err)
				}

				buffer.Finish()

				content, err := buffer.Bytes(0, 1000)
				require.NoError(t, err)
				assert.Equal(t, tc.expected, string(content))

				buffer.Close()
			}()
		})
	}
}
