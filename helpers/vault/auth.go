package vault

//go:generate mockery --inpackage --name AuthMethod

type AuthMethod interface {
	Name() string
	Authenticate(client Client) error
	Token() string
}
