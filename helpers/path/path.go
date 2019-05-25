package path_helpers

type Path interface {
	Join(elem ...string) string
	IsAbs(path string) bool
	IsRoot(path string) bool
	Contains(basepath, targetpath string) bool
}
