package path_helpers

import "path"

type unixPath struct{}

func (p *unixPath) Join(elem ...string) string {
	return path.Join(elem...)
}

func (p *unixPath) IsAbs(_path string) bool {
	_path = path.Clean(_path)
	return path.IsAbs(_path)
}

func (p *unixPath) IsRoot(_path string) bool {
	_path = path.Clean(_path)
	return path.IsAbs(_path) && path.Dir(_path) == _path
}

func (p *unixPath) Contains(basepath, targetpath string) bool {
	basepath = path.Clean(basepath)
	targetpath = path.Clean(targetpath)

	for {
		if targetpath == basepath {
			return true
		}
		if p.IsRoot(targetpath) || targetpath == "." {
			return false
		}
		targetpath = path.Dir(targetpath)
	}
}

func NewUnixPath() Path {
	return &unixPath{}
}
