package archives

import (
	"io"
	"os"

	"github.com/klauspost/compress/zip"
)

func createZipUIDGidField(w io.Writer, fi os.FileInfo) (err error) {
	// TODO: currently not supported
	return nil
}

func processZipUIDGidField(data []byte, file *zip.FileHeader) error {
	// TODO: currently not supported
	return nil
}
