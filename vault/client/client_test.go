package client

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/hashicorp/vault/api"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.com/gitlab-org/gitlab-runner/vault/config"
)

func TestNew(t *testing.T) {
	testURL := "https://localhost:1234"

	tests := map[string]struct {
		serverConfig  config.VaultServer
		expectedError string
	}{
		"fails TLS configuration": {
			serverConfig: config.VaultServer{
				URL:       testURL,
				TLSCAFile: "/tmp/not-existing",
			},
			expectedError: "couldn't prepare TLS configuration for the new Vault client: Error loading CA File: open /tmp/not-existing: no such file or directory",
		},
		"fails client initialization": {
			serverConfig: config.VaultServer{
				URL: ":",
			},
			expectedError: "couldn't create new Vault client: parse :: missing protocol scheme",
		},
		"creates client properly": {
			serverConfig: config.VaultServer{
				URL: "http://127.0.0.1:8200/",
			},
		},
	}

	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			cli, err := New(test.serverConfig)

			if test.expectedError != "" {
				assert.EqualError(t, err, test.expectedError)
				assert.Nil(t, cli)
				return
			}

			assert.NoError(t, err)
			assert.NotNil(t, cli)
		})
	}
}

func TestClient_IsServerReady(t *testing.T) {
	tests := map[string]struct {
		healthOutput   string
		expectedStatus bool
		expectedError  string
	}{
		"request returns an error": {
			healthOutput:   `abc`,
			expectedStatus: false,
			expectedError:  "invalid character 'a' looking for beginning of value",
		},
		"server is not initialized": {
			healthOutput:   `{"initialized":false, "sealed":true}`,
			expectedStatus: false,
		},
		"server is sealed": {
			healthOutput:   `{"initialized":true, "sealed":true}`,
			expectedStatus: false,
		},
		"server is ready": {
			healthOutput:   `{"initialized":true, "sealed":false}`,
			expectedStatus: true,
		},
	}

	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/v1/sys/health" {
					http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
					return
				}

				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(test.healthOutput))
			}))
			defer server.Close()

			cli, err := New(config.VaultServer{
				URL: server.URL,
			})
			require.NoError(t, err)

			resp := cli.IsServerReady()
			assert.Equal(t, test.expectedStatus, resp.State)
			if test.expectedError != "" {
				assert.EqualError(t, resp.Err, test.expectedError)
			} else {
				assert.NoError(t, resp.Err)
			}
		})
	}
}

func TestClient_SetToken(t *testing.T) {
	c := new(client)
	c.c = new(api.Client)

	assert.Empty(t, c.c.Token())
	c.SetToken("test-token")
	assert.Equal(t, "test-token", c.c.Token())
}

func TestClient_TokenLookupSelf(t *testing.T) {
	testToken := "test-token"

	tests := map[string]struct {
		response      string
		expectedError string
	}{
		"failure on API cal": {
			response:      "abc",
			expectedError: "error while executing self-lookup API: invalid character 'a' looking for beginning of value",
		},
		"failure on TokenTTL parsing": {
			response:      `{"data":{"ttl":"q"}}`,
			expectedError: `couldn't retrieve token's TTL: strconv.ParseInt: parsing "q": invalid syntax`,
		},
		"successful token self-lookup": {
			response: `{"data":{"ttl":"123"}}`,
		},
	}

	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/v1/auth/token/lookup-self" {
					http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
					return
				}

				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(test.response))
			}))
			defer server.Close()

			cli, err := New(config.VaultServer{
				URL: server.URL,
			})
			require.NoError(t, err)
			cli.SetToken(testToken)

			tokenInfo, err := cli.TokenLookupSelf()

			if test.expectedError != "" {
				assert.EqualError(t, err, test.expectedError)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, testToken, tokenInfo.Token)
			assert.Equal(t, 123*time.Second, tokenInfo.TTL)
		})
	}
}

func TestClient_UserpassLogin(t *testing.T) {
	testPath := "test-path"
	testUser := "test-user"
	testPass := "test-pass"

	testToken := "test-token"

	tests := map[string]struct {
		response      string
		expectedError string
	}{
		"failure on API cal": {
			response:      fmt.Sprintf(`{"auth":{"client_token":%q,"lease_duration":"q"}}`, testToken),
			expectedError: "error while executing userpass login: json: cannot unmarshal string into Go struct field SecretAuth.lease_duration of type int",
		},
		"successful userpass login": {
			response: fmt.Sprintf(`{"auth":{"client_token":%q,"lease_duration":123}}`, testToken),
		},
	}

	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != fmt.Sprintf("/v1/auth/%s/login/%s", testPath, testUser) {
					http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
					return
				}

				data, err := ioutil.ReadAll(r.Body)
				require.NoError(t, err)
				_ = r.Body.Close()

				var passData = struct {
					Password string `json:"password"`
				}{}

				err = json.Unmarshal(data, &passData)
				require.NoError(t, err)

				if passData.Password != testPass {
					w.WriteHeader(http.StatusBadRequest)
					return
				}

				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(test.response))
			}))
			defer server.Close()

			cli, err := New(config.VaultServer{
				URL: server.URL,
			})
			require.NoError(t, err)

			tokenInfo, err := cli.UserpassLogin("test-path", "test-user", "test-pass")

			if test.expectedError != "" {
				assert.EqualError(t, err, test.expectedError)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, testToken, tokenInfo.Token)
			assert.Equal(t, 123*time.Second, tokenInfo.TTL)
		})
	}
}

var (
	tlsCACertPEM = `-----BEGIN CERTIFICATE-----
MIIDADCCAeigAwIBAgIEXNC5qzANBgkqhkiG9w0BAQsFADAgMR4wHAYDVQQDExVW
YXVsdCBUZXN0IFNlcnZpY2UgQ0EwHhcNMTkwNTA2MjI0ODEwWhcNMjkwNTA2MjI0
ODEwWjAgMR4wHAYDVQQDExVWYXVsdCBUZXN0IFNlcnZpY2UgQ0EwggEiMA0GCSqG
SIb3DQEBAQUAA4IBDwAwggEKAoIBAQCU1ISdVuTWtsl4yyRFF7xKkhvb0lfokM+S
0XrgEndMgM5ffYoCFKDIqRN0wF13maIBOivBuADEUAb9XAPoum8MYqZcZABBsQ7F
ciamLUApoMDG5iyAA3vdV63Nh3gHf7pHZa3Iw9FABm1mPYxH/RJLzPVstLBHmanY
LCsS4AtHTjeI2Xi4ADQ7pojze2mkM+Sir3bDh0X5ruIKEW5vrugqb7Z9AY1aGjF7
DfuYH2xZTDrKzGM4YvELpUqtBQhzMFXYUXvIPnx3ebpeGcfr+pT6uXp7pAWy022c
359UN3784Wf24WSysZvcfKcEa5w2k6PRC+ifGK4DAklHzP+mr6J3AgMBAAGjQjBA
MA4GA1UdDwEB/wQEAwIChDAdBgNVHSUEFjAUBggrBgEFBQcDAgYIKwYBBQUHAwEw
DwYDVR0TAQH/BAUwAwEB/zANBgkqhkiG9w0BAQsFAAOCAQEAR1nRzadilnpEXKet
xe22117UBy9AUBU7kYNzuAD+PSiKzVA3Kwjj8s1j1YC3n5+3HHdkN1J52FBSC+1W
cOGurQ5eoHqC9oB9K9e3Ix19gnzaJnMmmrjXwQ4LCaG7I0yDXU4ZHQvbRpqXfhym
TYIyQqzxrxzQmbZQHtM4bZMFJJHqB/yXAEgS+ITZb1lNFjwDQRl7uHS2es/cKdT3
vifopH80DI/TZmjPsvssodKwLVY28vFxvQQd6obfRqx/WyebbZiUGGlLwLDHlD5C
1APGmfq3vnGz2KbGp327cSMJKFPcmD+j6tyDgc7yMOVBsNyTF6SnzKTodN5C3apY
nQkeRw==
-----END CERTIFICATE-----`
	serverTLSCertPEM = `-----BEGIN CERTIFICATE-----
MIIC8DCCAdigAwIBAgIEXNC5rDANBgkqhkiG9w0BAQsFADAgMR4wHAYDVQQDExVW
YXVsdCBUZXN0IFNlcnZpY2UgQ0EwHhcNMTkwNTA2MjI0ODEwWhcNMjAwNTA2MjI0
ODEwWjAQMQ4wDAYDVQQDEwV2YXVsdDCCASIwDQYJKoZIhvcNAQEBBQADggEPADCC
AQoCggEBAPMIfa46m8qCyLVFSpFySiRrnVFpMdV2eLdSXJe8pKcxqAYxRlnY/NEz
4wR4Xvz0XGHkDA6jM18shuEUEN7q1H1y1Gv/qzit6qgLCp/+dJ3fHdqp0sW5M50U
GcoliX4n8uJfVlk/iOLP4VwCamAvyD0Oeu23l6JAdrjgBHgg7z34yaj2TK2EoUzb
tsNMa+bbNWoaAxJszpsRa6PpSS3xH66mcjcG9oj9KwX/w5pWqguQ3PxHc78QPs97
dab2468FqVCVtaLQQkJSgF2mq76vNyE+1z+GhHM/57FSJPGnhCztkWOhhe045aT3
uftXKaBb5nNdgcMpCQBHBKj00pn9M/ECAwEAAaNCMEAwDgYDVR0PAQH/BAQDAgeA
MB0GA1UdJQQWMBQGCCsGAQUFBwMCBggrBgEFBQcDATAPBgNVHREECDAGhwR/AAAB
MA0GCSqGSIb3DQEBCwUAA4IBAQBSJszPC69lsdoqZnfM2SiRxBlZcCT5qIIjWLwo
dx+nzMtYZruqSEjcjVw+sdMLJEqY1Mu4CcYTY9+T/+4CbwY3z3XmvQ4Tyx+ABNhZ
b8L8Ic09MhtHMUtYDmXSs+P3gce38AUS3p4fJEo9ljcZPLW4AOrWtwgdWkHlwNDC
NUmo/y0dsm2EuvyGBBMk5a0ZJgFdLs/AuT7TqJv0oGlj+kQtM2bKvPRKRbr+EsE7
3CdrhIBjp6oaaCkdQX+jNV1ZyTAUH/bgbydE6VEc8l7U1KV9GWhscdeT2QieHuY/
hZyzeJfrEKxsylr4bqTiqYD2lNMP6md6+bskkwMjX2vOb069
-----END CERTIFICATE-----`
	serverTLSKeyPEM = `-----BEGIN RSA PRIVATE KEY-----
MIIEpAIBAAKCAQEA8wh9rjqbyoLItUVKkXJKJGudUWkx1XZ4t1Jcl7ykpzGoBjFG
Wdj80TPjBHhe/PRcYeQMDqMzXyyG4RQQ3urUfXLUa/+rOK3qqAsKn/50nd8d2qnS
xbkznRQZyiWJfify4l9WWT+I4s/hXAJqYC/IPQ567beXokB2uOAEeCDvPfjJqPZM
rYShTNu2w0xr5ts1ahoDEmzOmxFro+lJLfEfrqZyNwb2iP0rBf/DmlaqC5Dc/Edz
vxA+z3t1pvbjrwWpUJW1otBCQlKAXaarvq83IT7XP4aEcz/nsVIk8aeELO2RY6GF
7TjlpPe5+1cpoFvmc12BwykJAEcEqPTSmf0z8QIDAQABAoIBAQDMWvBrR2bmcvBX
1EruYB7N3xtqcDzyFGtPItcu0/XTjKKPinFwbU+wjaOvh5O/ua3QtlQZHsu8lJFZ
w2ioOOeyEJNjuJj90OfGo3osrGbctNbCnhfYIHGw/EzvOH8TcH4AMVBHPXBZ35jM
qE9QT/1cscdWChFb4j6yF9RKOs9Q2DUZV+q/GrVamspFEzrH9gkftnwqwp8ObP22
QM65dXXjKQdaaxZNguE1zlXtcUG/jKB9A5eSJ3Q+aSOzHbHsUsOyY/XkR3NYiraP
5b1/Hh8g/k1MVrjx4EMfYdrUTf5bQ2tQZsSnOSuQcsSKzpa1Hm5Im3zqJjfnCohc
LnYuEqCtAoGBAP1xZUDUoknRMY8/EyLKzqnAiN8a9ADpd5pYmVJt45KACOQ7qLry
v57CGYRQ0VUp6ffImzQWihgr0xwZcYQSJ9eylgJHoY5RB3TqnGw3m937MOFQiHKP
9jTcN0B+o+RqNVqnqQOc3DCL0YFE8DI2589QINp8S8tqY1ATDFstbgk/AoGBAPV8
NWKl/sGUkr3Yg6IJopOUWRyQtHdzHlk+bF3Z82Z7bLbJNWB2/A6Myh6yAbkzvH8r
aaqhr4uSpz0w2I5G3Gpwkm6r1g3uQ5Vj50/mKV8W9/QcC2RR0UcYcRkcKZ3ZodyQ
12oIKZgIdzrdh90RkIFNHzQADoin3GVrDKhw48bPAoGBANhdT1CqdqXIJqQg9+gy
9X1r9i1pqDeDGO02iCYb1DVEgtK9r81x4W7aS8hu6lbnQmub4gv01g3OlBqgCg3z
Jfp55qCpoF2MBW6lv8aPLsyyXkdsZiBPkKQOAElaE/azSTtMePixmDUFmGTggqKL
xxhwUqvTgy10dLZunJTWUuMnAoGAI5Wpt28QiscarmJgUnDLHFF4yWdAgcAyOgWO
d9xMKCLkE2r/TchxqTpHYkOzdEFHpbeJTa66X6UWkQwvmBA1i0heMaS/Fq3fJhyh
PzfB74LI1p3qGNSzXXbxjg5DChquF+b3Euuz+9HeVq4eL7GIHPYs+8C2WqDalej6
oMAchIkCgYB59rI6YsMGiZ7CxqTd84Qg+G/FpfwLw6yiWh49nq/rRmr5C3V4I+Xs
ZT60RHTNWQzcWBmdUuUZaGAVZZ33EZnSvieEjfwsVT9PV5o4tHPTmeYIZV2rgyLa
E7vUyTQSqBqEvGXaTWtog17kpLbFRVHNSe0HYjMFqYiFdBVFtXzeJA==
-----END RSA PRIVATE KEY-----`
	tlsCertPEM = `-----BEGIN CERTIFICATE-----
MIIC8DCCAdigAwIBAgIEXNC5rTANBgkqhkiG9w0BAQsFADAgMR4wHAYDVQQDExVW
YXVsdCBUZXN0IFNlcnZpY2UgQ0EwHhcNMTkwNTA2MjI0ODEyWhcNMjAwNTA2MjI0
ODEyWjAQMQ4wDAYDVQQDEwVjZXJ0MTCCASIwDQYJKoZIhvcNAQEBBQADggEPADCC
AQoCggEBAKIBMdxkRersvBHbs7weqMIUngUDzvG0fEMm+1lzYTsJvmQf25Xn9LrM
LN3m8PTURy/mSdhjQClNPnnDwGmQsoiLsvKOibF8qiSZHusAWHNrSzrw5FT5rE+w
yE2uscSEYeH+6+uVuZQ4jaoH6P3PSVEynwRGWMvW2MDn9CkExBiXamnr4JCD9tFU
E+Iliu/icKDDQocj9f1nOKCfR14l/REdw5/aqy9deUZvAAL0UVZH9HFwlxFYowRM
lL7s0FzisLg0wJ5N5xhSctSdfuOuhUdhwzJQK7RlNJDUYrYW+8M68T6Iaq8qkD2o
ciMRp786XW46o8i4qSF67/+dpVRmEn0CAwEAAaNCMEAwDgYDVR0PAQH/BAQDAgeA
MB0GA1UdJQQWMBQGCCsGAQUFBwMCBggrBgEFBQcDATAPBgNVHREECDAGhwR/AAAB
MA0GCSqGSIb3DQEBCwUAA4IBAQBsTFcx3+uLNb54UcfW1LL2bziMQSo9D6z7HxZO
iqBNW9uYUUEii3HEss6bU4N7RDCnh5+ZWDrGYibZXsdC1/UxUWJ5J3AvUntdDTEJ
c21g4xofwWDd5TdGcyOqbopGUs/kqGw61Dl4z1x6FwdTqUZag2RmCHvCM9H2RZUo
Svn+XGiuvsn2RXRjxZ0vZNiyTdTL2IdmiswTMQkncm8vpW/tEVwmMzNseKordINC
lP4gKCU/SmywFKt6zfvVDrIYWuMx54mslbciFUda49iiT7rWwdhHO+0Bp1uqG0W/
uDAFIAASSoKa4Xv6ov/4AhszGJ4gzn1/bikqfDD4MHPWFi7P
-----END CERTIFICATE-----`
	tlsKeyPEM = `-----BEGIN RSA PRIVATE KEY-----
MIIEpQIBAAKCAQEAogEx3GRF6uy8EduzvB6owhSeBQPO8bR8Qyb7WXNhOwm+ZB/b
lef0usws3ebw9NRHL+ZJ2GNAKU0+ecPAaZCyiIuy8o6JsXyqJJke6wBYc2tLOvDk
VPmsT7DITa6xxIRh4f7r65W5lDiNqgfo/c9JUTKfBEZYy9bYwOf0KQTEGJdqaevg
kIP20VQT4iWK7+JwoMNChyP1/Wc4oJ9HXiX9ER3Dn9qrL115Rm8AAvRRVkf0cXCX
EVijBEyUvuzQXOKwuDTAnk3nGFJy1J1+466FR2HDMlArtGU0kNRithb7wzrxPohq
ryqQPahyIxGnvzpdbjqjyLipIXrv/52lVGYSfQIDAQABAoIBAQCFwpTHkqdD7Ajw
ecUx+uJ9tIYwP8+7M7kxvNrlJWXPWCEyDDfC0wz2uqQE56xliWvpeavZFUGhmFyQ
LvcMcmNmaGns6ZF40SSuKRslD1j4m0s3NDRikO8bsSwBL88pIeCrt05Van8aiYM2
M2fFQEQZ3cD7x5WDYDYBOMMYpw29+t50tQNwW+Ub+YIiH3AAF8BRDWdrxWOZkQfr
z2MnkimvT0gg9ozhgZeLGw+Gst4Wa+AIYH7GupdEf4M1R7Rn2jY9riKEQOIPZL0h
/A/kz9H4TNXj/y7g5YMerTEdhxmw7HmG2dYQnKH9DbnnOLf0OBeACg4Y2iYn8t5+
jz72HGylAoGBANIuYcVOiFNnEx+KD5TuNfpzSw6z1QeiY0G56N8ItHN3o/UL5HJW
BQJea/Vdx34lP0DqB35byDAbzNvfukLuREiwwf7fqU+HO3OqkvjCzR5SSg2yijL8
5GnBfwIDDZRgVHLujcXryz4ksAL35pmXOltx8zfwCRHHedbgNzHyQ3MLAoGBAMVS
N75X88S41nQ9SClDu+lD5o5OJmmz6bvABVSddlaY0Emv87ciPSNNHF+I8+XNTyAC
jAK8wiXwWnsiEh9ykivoaTkPIAXUy92fZJ4EFFPu8+KD+V/nlSKxPiZA87YLSTCS
IZNWM2szqNIn6j+/PuOWi5qhwysKC5blCiQy2gWXAoGBALrdp+l/L+9O9g6VddMI
kw8v0CyrMByQgMTf4C3jlGQQm8HzJ9GLrvpzLnLBROtffERfjfgG7A3xuYpG+Fgn
dKhYFrJe8i4V4oKsxezLbQinStWwxfQdKYrpEN2eD0W6+3oPpBay1ElU3vRUqT4m
2SiSQBacn8Oh4S5svEX4yYUPAoGAHLYZ5lhl3/oFOmSwW1C/xvFaWtqEPF0xZWBL
ZkSDM5aIuDAiBkO1Ia3Wsw/6bTWyjbXRKZTNqzeN8tzCRlElc74dkW/h+Pc9ssG+
oj91tcDPO+Z4IrxPtvyTTn2k+JgrziV1PTsNwEuEBRBJxXzOac8+AQIIo/qSNSKe
lyXPE4ECgYEArs/XkrNgNcHgSr176tB4LcrgOaogTDiEh2R9fR6tvzMSdLUOjHAj
cx5FUbdFSsBf2rc6/KPS+Xxz5PyKrheTHyHnHOyLmlMzwZUoellJ2k8OV9jJJTmO
2LJpUBgJNV/9fV/bzwt7yTzfpyeVborddV4tqiGZEZ20Jmhe5aJGahU=
-----END RSA PRIVATE KEY-----`
)

func TestClient_TLSLogin(t *testing.T) {
	testPath := "test-path"
	testRoleName := "test-name"
	testToken := "test-token"

	tests := map[string]struct {
		certPEM       string
		keyPEM        string
		response      string
		expectedError string
	}{
		"invalid TLS configuration": {
			expectedError: "couldn't re-create the Vault client with TLS Client Authentication config: couldn't prepare TLS configuration for the new Vault client: tls: failed to find any PEM data in certificate input",
		},
		"failure on API cal": {
			certPEM:       tlsCertPEM,
			keyPEM:        tlsKeyPEM,
			response:      fmt.Sprintf(`{"auth":{"client_token":%q,"lease_duration":"q"}}`, testToken),
			expectedError: "error while executing TLS login: json: cannot unmarshal string into Go struct field SecretAuth.lease_duration of type int",
		},
		"successful userpass login": {
			certPEM:  tlsCertPEM,
			keyPEM:   tlsKeyPEM,
			response: fmt.Sprintf(`{"auth":{"client_token":%q,"lease_duration":123}}`, testToken),
		},
	}

	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			caCertFile, err := createTLSFile("ca.cert", tlsCACertPEM)
			require.NoError(t, err)
			certFile, err := createTLSFile("client.cert", test.certPEM)
			require.NoError(t, err)
			keyFile, err := createTLSFile("client.key", test.keyPEM)
			require.NoError(t, err)

			server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != fmt.Sprintf("/v1/auth/%s/login", testPath) {
					http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
					return
				}

				data, err := ioutil.ReadAll(r.Body)
				require.NoError(t, err)
				_ = r.Body.Close()

				var nameData = struct {
					Name string `json:"name"`
				}{}

				err = json.Unmarshal(data, &nameData)
				require.NoError(t, err)

				if nameData.Name != testRoleName {
					w.WriteHeader(http.StatusBadRequest)
					return
				}

				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(test.response))
			}))

			caCertificate, err := tls.X509KeyPair([]byte(serverTLSCertPEM), []byte(serverTLSKeyPEM))
			require.NoError(t, err)

			pool := x509.NewCertPool()
			ok := pool.AppendCertsFromPEM([]byte(tlsCACertPEM))
			if !ok {
				assert.Fail(t, "couldn't add CA certificate to ClientCAa pool")
			}

			server.TLS = &tls.Config{
				Certificates: []tls.Certificate{caCertificate},
				ClientAuth:   tls.VerifyClientCertIfGiven,
				ClientCAs:    pool,
			}

			server.StartTLS()
			defer server.Close()

			cli, err := New(config.VaultServer{
				URL:       server.URL,
				TLSCAFile: caCertFile,
			})
			require.NoError(t, err)

			tokenInfo, err := cli.TLSLogin(testPath, testRoleName, certFile, keyFile)

			if test.expectedError != "" {
				assert.EqualError(t, err, test.expectedError)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, testToken, tokenInfo.Token)
			assert.Equal(t, 123*time.Second, tokenInfo.TTL)
		})
	}
}

func createTLSFile(name string, data string) (string, error) {
	file, err := ioutil.TempFile("", name)
	if err != nil {
		return "", errors.Wrap(err, "error while creating temporary file")
	}

	buf := bytes.NewBufferString(data)
	bufLen := buf.Len()

	n, err := io.Copy(file, buf)
	if err != nil {
		return "", errors.Wrapf(err, "error while writing to temporary file %q", file.Name())
	}

	if n != int64(bufLen) {
		return "", errors.Wrapf(err, "length of data written to %q doesn't equal to the length of provided data", file.Name())
	}

	return file.Name(), nil
}

func TestClient_Read(t *testing.T) {
	tests := map[string]struct {
		path          string
		response      string
		expectedError string
	}{
		"failure on secret read": {
			path:          "error",
			expectedError: `couldn't read data for "error": Error making API request.`,
		},
		"proper secret read": {
			path:     "test/path",
			response: `{"data":{"testKey":"testValue"}}`,
		},
	}

	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/v1/error" {
					http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
					return
				}

				if r.URL.Path != "/v1/test/path" {
					http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
					return
				}

				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(test.response))
			}))
			defer server.Close()

			cli, err := New(config.VaultServer{
				URL: server.URL,
			})
			require.NoError(t, err)

			data, err := cli.Read(test.path)

			if test.expectedError != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), test.expectedError)
				return
			}

			assert.NoError(t, err)
			require.Contains(t, data, "testKey")
			assert.Equal(t, "testValue", data["testKey"])
		})
	}
}
