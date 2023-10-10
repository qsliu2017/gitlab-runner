package resourcecheck

type ResourceChecker interface {
	Exists() error
}
