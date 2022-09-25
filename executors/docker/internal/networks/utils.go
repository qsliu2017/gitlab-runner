package networks

//go:generate mockery --inpackage --name debugLogger

type debugLogger interface {
	Debugln(args ...interface{})
}
