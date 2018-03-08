package network

type JobCredentials struct {
	ID          int    `long:"id" env:"CI_JOB_ID" description:"The build ID to upload artifacts for"`
	Token       string `long:"token" env:"CI_JOB_TOKEN" required:"true" description:"Build token"`
	URL         string `long:"url" env:"CI_SERVER_URL" required:"true" description:"GitLab CI URL"`
	TLSCAFile   string `long:"tls-ca-file" env:"CI_SERVER_TLS_CA_FILE" description:"File containing the certificates to verify the peer when using HTTPS"`
	TLSCertFile string `long:"tls-cert-file" env:"CI_SERVER_TLS_CERT_FILE" description:"File containing certificate for TLS client auth with runner when using HTTPS"`
	TLSKeyFile  string `long:"tls-key-file" env:"CI_SERVER_TLS_KEY_FILE" description:"File containing private key for TLS client auth with runner when using HTTPS"`
}

func (j *JobCredentials) GetURL() string {
	return j.URL
}

func (j *JobCredentials) GetTLSCAFile() string {
	return j.TLSCAFile
}

func (j *JobCredentials) GetTLSCertFile() string {
	return j.TLSCertFile
}

func (j *JobCredentials) GetTLSKeyFile() string {
	return j.TLSKeyFile
}

func (j *JobCredentials) GetToken() string {
	return j.Token
}
