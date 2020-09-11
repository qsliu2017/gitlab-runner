package trace

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTraceLimit(t *testing.T) {
	traceMessage := "This is the long message"

	buffer, err := New()
	require.NoError(t, err)
	defer buffer.Close()

	buffer.SetLimit(10)
	assert.Equal(t, 0, buffer.Size())

	for i := 0; i < 100; i++ {
		_, err = buffer.Write([]byte(traceMessage))
		require.NoError(t, err)
	}

	buffer.Finish()

	content, err := buffer.Bytes(0, 1000)
	require.NoError(t, err)

	assert.Equal(t, 61, buffer.Size())
	assert.Equal(t, "crc32:597f1ee1", buffer.Checksum())
	assert.Equal(t, "This is th\n\x1b[31;1mJob's log exceeded limit of 10 bytes.\x1b[0;m\n", string(content))
}

func BenchmarkTrace10kWithURLScrub(b *testing.B) {
	logLine := []byte("hello world, this is a lengthy log line including secrets such as 'hello', and " +
		"https://example.com/?rss_token=foo&rss_token=bar and http://example.com/?authenticity_token=deadbeef and " +
		"https://example.com/?rss_token=foobar. it's longer than most log lines, but probably a good test for " +
		"anything that's benchmarking how fast it is to write log lines.")

	for i := 0; i < b.N; i++ {
		func() {
			buffer, err := New()
			require.NoError(b, err)
			defer buffer.Close()

			buffer.SetLimit(int(^uint(0) >> 1))
			buffer.SetMasked([]string{"hello"})

			b.ReportAllocs()
			b.SetBytes(int64(len(logLine) * 10000))
			for i := 0; i < 10000; i++ {
				_, _ = buffer.Write(logLine)
			}
			buffer.Finish()
		}()
	}
}
