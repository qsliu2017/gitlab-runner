package config

type Vault struct {
	Server  VaultServer  `toml:"server" json:"server" group:"vault server configuration" namespace:"server"`
}

type VaultServer struct {
	URL       string `toml:"url" json:"url" long:"url" env:"RUNNER_VAULT_URL" required:"true" description:"URL of the Vault server to use"`
	TLSCAFile string `toml:"tls_ca_file,omitempty" json:"tls_ca_file" long:"tls-ca-file" env:"RUNNER_VAULT_TLS_CA_FILE" description:"CA file to use when communicating with Vault server"`
}

