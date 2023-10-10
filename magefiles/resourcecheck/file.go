package resourcecheck

import "os"

type File struct {
	file string
}

func NewFile(f string) File {
	return File{file: f}
}

func (f File) Exists() error {
	_, err := os.Stat(f.file)
	return err
}
