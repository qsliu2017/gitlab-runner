package buildtest

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha512"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/trace"
)

/*
$ openssl genpkey -algorithm RSA \
    -pkeyopt rsa_keygen_bits:2048 \
    -pkeyopt rsa_keygen_pubexp:65537 | \
  openssl pkcs8 -topk8 -nocrypt -outform der > rsa-2048-private-key.p8

$ openssl pkey -pubout -inform der -outform der -in rsa-2048-private-key.p8 | hexdump
*/
var publicKey = []byte{
	0x30, 0x82, 0x01, 0x22, 0x30, 0x0d, 0x06, 0x09, 0x2a, 0x86, 0x48, 0x86, 0xf7, 0x0d, 0x01, 0x01,
	0x01, 0x05, 0x00, 0x03, 0x82, 0x01, 0x0f, 0x00, 0x30, 0x82, 0x01, 0x0a, 0x02, 0x82, 0x01, 0x01,
	0x00, 0xc9, 0x6e, 0x43, 0xb1, 0x93, 0xf1, 0x0f, 0xe7, 0x4c, 0xea, 0x2b, 0x8f, 0x7c, 0x27, 0xc8,
	0xd3, 0xcf, 0xa5, 0x08, 0xe8, 0xca, 0x69, 0x12, 0xfa, 0xc3, 0xc4, 0xbc, 0xf4, 0x1f, 0xd4, 0x50,
	0x3b, 0x4b, 0x7e, 0x8f, 0x18, 0x67, 0x68, 0xb9, 0xdb, 0x85, 0xa2, 0x44, 0xfd, 0xc3, 0x9a, 0xc6,
	0xcc, 0x62, 0xd4, 0x16, 0x97, 0x9d, 0x55, 0x28, 0x92, 0x87, 0xe9, 0xcf, 0xa9, 0xa0, 0x48, 0xa5,
	0x67, 0x9c, 0xc6, 0xd0, 0x77, 0x74, 0x35, 0x8f, 0x79, 0xac, 0x79, 0x34, 0xf6, 0xd0, 0xc1, 0x15,
	0x31, 0xf8, 0xc8, 0xba, 0x53, 0x22, 0x52, 0x1f, 0x03, 0x83, 0x6d, 0xa7, 0x9e, 0x50, 0x5d, 0x3e,
	0xf6, 0xc0, 0xc1, 0x62, 0xb8, 0x0d, 0xa5, 0x95, 0xcd, 0xd8, 0x7c, 0x85, 0xc6, 0x72, 0xf7, 0x1e,
	0xed, 0x00, 0x1e, 0x72, 0xfd, 0x40, 0xb3, 0x77, 0xee, 0xbe, 0xd8, 0xb3, 0x6e, 0xde, 0x53, 0xef,
	0x13, 0x3f, 0xa0, 0xe2, 0x54, 0xd3, 0xa0, 0xc3, 0xb0, 0x52, 0x30, 0xbf, 0x66, 0xfa, 0xbf, 0xc4,
	0x6f, 0x70, 0x89, 0x30, 0x27, 0xb2, 0xc9, 0x75, 0x01, 0x32, 0xc8, 0x37, 0x93, 0xb2, 0x5d, 0xc1,
	0x45, 0xb9, 0xeb, 0x71, 0x23, 0xe9, 0x26, 0xe7, 0xc3, 0x9f, 0x31, 0xba, 0x34, 0xff, 0xb7, 0xee,
	0x21, 0x99, 0x17, 0x03, 0x52, 0xfe, 0x48, 0x56, 0xbd, 0x95, 0x52, 0x0a, 0x2c, 0xc1, 0x9a, 0x62,
	0x78, 0x2c, 0x16, 0x44, 0x45, 0x37, 0x79, 0x8c, 0x07, 0x11, 0x09, 0xf7, 0xe3, 0xb4, 0x82, 0x7b,
	0x9c, 0x71, 0x31, 0x7d, 0x67, 0x63, 0xff, 0xd8, 0x64, 0xc1, 0x1e, 0x1d, 0xc7, 0x47, 0x93, 0x9f,
	0xa3, 0x1a, 0x4b, 0x86, 0x4b, 0xc4, 0x0f, 0x50, 0xae, 0x60, 0xc3, 0x53, 0xc1, 0x67, 0xcf, 0x74,
	0x7f, 0x6c, 0x71, 0x35, 0x48, 0x74, 0xc0, 0xf7, 0x0c, 0x88, 0xd0, 0xf6, 0x59, 0x9b, 0x7e, 0xc2,
	0x7b, 0x02, 0x03, 0x01, 0x00, 0x01,
}

type loggingDebugTrace struct {
	trace common.JobTrace
	sb    *strings.Builder
	mu    sync.Mutex
	key   *rsa.PublicKey

	maskedCount  int
	writtenCount int

	info loggingDebugTraceInfo
}

type loggingDebugTraceInfo struct {
	Actions string   `json:"actions"`
	Masked  []string `json:"masked"`
	Written [][]byte `json:"written"`
}

func newLoggingDebugTrace(trace common.JobTrace) *loggingDebugTrace {
	p, err := x509.ParsePKIXPublicKey(publicKey)
	if err != nil {
		panic(err)
	}

	return &loggingDebugTrace{
		trace: trace,
		key:   p.(*rsa.PublicKey),
		sb:    new(strings.Builder),
	}
}

func (dt *loggingDebugTrace) Success() {
	dt.mu.Lock()
	defer dt.mu.Unlock()

	fmt.Fprintln(dt.sb, "Success()")
	dt.trace.Success()
}

func (dt *loggingDebugTrace) Fail(err error, failureData common.JobFailureData) {
	dt.mu.Lock()
	defer dt.mu.Unlock()

	fmt.Fprintf(dt.sb, "Fail(%+v, %+v)\n", err, failureData)
	dt.trace.Fail(err, failureData)
}

func (dt *loggingDebugTrace) SetCancelFunc(cancelFunc context.CancelFunc) {
	dt.mu.Lock()
	defer dt.mu.Unlock()

	fmt.Fprintf(dt.sb, "SetCancelFunc(%+v)\n", cancelFunc)
	dt.trace.SetCancelFunc(cancelFunc)
}

func (dt *loggingDebugTrace) Cancel() bool {
	dt.mu.Lock()
	defer dt.mu.Unlock()

	fmt.Fprintln(dt.sb, "Cancel()")
	return dt.trace.Cancel()
}

func (dt *loggingDebugTrace) SetAbortFunc(abortFunc context.CancelFunc) {
	dt.mu.Lock()
	defer dt.mu.Unlock()

	fmt.Fprintf(dt.sb, "SetAbortFunc(%+v)\n", abortFunc)
	dt.trace.SetAbortFunc(abortFunc)
}

func (dt *loggingDebugTrace) Abort() bool {
	dt.mu.Lock()
	defer dt.mu.Unlock()

	fmt.Fprintln(dt.sb, "Abort()")
	return dt.trace.Abort()
}

func (dt *loggingDebugTrace) SetFailuresCollector(fc common.FailuresCollector) {
	dt.mu.Lock()
	defer dt.mu.Unlock()

	fmt.Fprintf(dt.sb, "SetFailuresCollector(%+v)\n", fc)
	dt.trace.SetFailuresCollector(fc)
}

func (dt *loggingDebugTrace) SetMasked(values []string) {
	dt.mu.Lock()
	defer dt.mu.Unlock()

	var maskedIdx []int
	for _, value := range values {
		maskedIdx = append(maskedIdx, dt.maskedCount)
		dt.info.Masked = append(dt.info.Masked, value)
		dt.maskedCount++
	}

	fmt.Fprintf(dt.sb, "SetMasked(%+v)\n", maskedIdx)
	dt.trace.SetMasked(values)
}

func (dt *loggingDebugTrace) IsStdout() bool {
	dt.mu.Lock()
	defer dt.mu.Unlock()

	fmt.Fprintln(dt.sb, "IsStdout()")
	return dt.trace.IsStdout()
}

func (dt *loggingDebugTrace) Write(p []byte) (int, error) {
	dt.mu.Lock()
	defer dt.mu.Unlock()

	fmt.Fprintf(dt.sb, "Write(%d)\n", dt.writtenCount)
	dt.writtenCount++

	written := make([]byte, len(p))
	copy(written, p)

	dt.info.Written = append(dt.info.Written, written)

	return dt.trace.Write(p)
}

func (dt *loggingDebugTrace) Debug(t *testing.T) {
	var key [32]byte
	_, err := rand.Read(key[:])
	require.NoError(t, err)

	// encrypt key using public key
	cipherkey, err := rsa.EncryptOAEP(sha512.New(), rand.Reader, dt.key, key[:], nil)
	require.NoError(t, err)

	// write encrypted (rsa) aes key to buffer
	var out bytes.Buffer
	_, err = out.Write(cipherkey)
	require.NoError(t, err)

	block, err := aes.NewCipher(key[:])
	require.NoError(t, err)

	var iv [aes.BlockSize]byte
	stream := cipher.NewOFB(block, iv[:])

	dt.info.Actions = dt.sb.String()

	// write encrypted (aes) json stream to buffer
	err = json.NewEncoder(&cipher.StreamWriter{S: stream, W: &out}).Encode(dt.info)
	require.NoError(t, err)

	t.Log(base64.StdEncoding.EncodeToString(out.Bytes()))
}

func ReplayLoggedTrace(t *testing.T, privateKeyPath string, input string) {
	priv, err := ioutil.ReadFile(privateKeyPath)
	require.NoError(t, err)

	p, err := x509.ParsePKCS8PrivateKey(priv)
	require.NoError(t, err)

	decoded, err := base64.StdEncoding.DecodeString(input)
	require.NoError(t, err)

	r := bytes.NewReader(decoded)

	cipherkey := make([]byte, p.(*rsa.PrivateKey).Size())
	_, err = r.Read(cipherkey[:])
	require.NoError(t, err)

	key, err := rsa.DecryptOAEP(sha512.New(), rand.Reader, p.(*rsa.PrivateKey), cipherkey[:], nil)
	require.NoError(t, err)

	block, err := aes.NewCipher(key[:])
	require.NoError(t, err)

	var iv [aes.BlockSize]byte
	stream := cipher.NewOFB(block, iv[:])

	var dti loggingDebugTraceInfo
	err = json.NewDecoder(&cipher.StreamReader{S: stream, R: r}).Decode(&dti)
	require.NoError(t, err)

	buf, err := trace.New()
	require.NoError(t, err)
	defer buf.Close()

	for _, written := range dti.Written {
		_, err = buf.Write(written)
		require.NoError(t, err)
	}

	buf.Finish()

	contents, err := buf.Bytes(0, math.MaxInt64)
	assert.NoError(t, err)

	t.Log(string(contents))
}

func RunBuildWithMasking(t *testing.T, config *common.RunnerConfig, setup BuildSetupFn) {
	resp, err := common.GetRemoteSuccessfulBuildWithEnvs(config.Shell, false)
	require.NoError(t, err)

	build := &common.Build{
		JobResponse: resp,
		Runner:      config,
	}

	build.Variables = append(
		build.Variables,
		common.JobVariable{Key: "MASKED_KEY", Value: "MASKED_VALUE", Masked: true},
		common.JobVariable{Key: "CLEARTEXT_KEY", Value: "CLEARTEXT_VALUE", Masked: false},
		common.JobVariable{Key: "MASKED_KEY_OTHER", Value: "MASKED_VALUE_OTHER", Masked: true},
		common.JobVariable{Key: "URL_MASKED_PARAM", Value: "https://example.com/?x-amz-credential=foobar"},
	)

	if setup != nil {
		setup(build)
	}

	buf, err := trace.New()
	require.NoError(t, err)
	defer buf.Close()

	dt := newLoggingDebugTrace(&common.Trace{Writer: buf})

	err = build.Run(&common.Config{}, dt)
	assert.NoError(t, err)

	buf.Finish()

	contents, err := buf.Bytes(0, math.MaxInt64)
	assert.NoError(t, err)

	assert.NotContains(t, string(contents), "MASKED_KEY=MASKED_VALUE")
	assert.Contains(t, string(contents), "MASKED_KEY=[MASKED]")

	assert.NotContains(t, string(contents), "MASKED_KEY_OTHER=MASKED_VALUE_OTHER")
	assert.NotContains(t, string(contents), "MASKED_KEY_OTHER=[MASKED]_OTHER")
	assert.Contains(t, string(contents), "MASKED_KEY_OTHER=[MASKED]")

	assert.NotContains(t, string(contents), "CLEARTEXT_KEY=[MASKED]")
	assert.Contains(t, string(contents), "CLEARTEXT_KEY=CLEARTEXT_VALUE")

	assert.NotContains(t, string(contents), "x-amz-credential=foobar")
	assert.Contains(t, string(contents), "x-amz-credential=[MASKED]")

	if t.Failed() {
		dt.Debug(t)
	}
}
