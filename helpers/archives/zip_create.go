package archives

import (
	"archive/zip"
	"bufio"
	"compress/flate"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"

	"github.com/sirupsen/logrus"
)

var bufioPool = sync.Pool{
	New: func() interface{} {
		return bufio.NewReader(nil)
	},
}

func createZipDirectoryEntry(archive *zip.Writer, fh *zip.FileHeader) error {
	fh.Name += "/"
	_, err := archive.CreateHeader(fh)
	return err
}

func createZipSymlinkEntry(archive *zip.Writer, fh *zip.FileHeader) error {
	fw, err := archive.CreateHeader(fh)
	if err != nil {
		return err
	}

	link, err := os.Readlink(fh.Name)
	if err != nil {
		return err
	}

	_, err = io.WriteString(fw, link)
	return err
}

func createZipFileEntry(archive *zip.Writer, fh *zip.FileHeader, level int) error {
	fh.Method = zip.Deflate
	// If the Deflate compression level is flate.NoCompression, we may as well
	// set the zip compression method to zip.Store.
	if level == flate.NoCompression {
		fh.Method = zip.Store
	}

	fw, err := archive.CreateHeader(fh)
	if err != nil {
		return err
	}

	file, err := os.Open(fh.Name)
	if err != nil {
		return err
	}
	defer file.Close()

	br := bufioPool.Get().(*bufio.Reader)
	defer bufioPool.Put(br)
	br.Reset(file)

	_, err = io.Copy(fw, br)

	return err
}

func createZipEntry(archive *zip.Writer, fileName string, level int) error {
	fi, err := os.Lstat(fileName)
	if err != nil {
		logrus.Warningln("File ignored:", err)
		return nil
	}

	fh, err := zip.FileInfoHeader(fi)
	if err != nil {
		return err
	}
	fh.Name = fileName
	fh.Extra = createZipExtra(fi)

	switch fi.Mode() & os.ModeType {
	case os.ModeDir:
		return createZipDirectoryEntry(archive, fh)

	case os.ModeSymlink:
		return createZipSymlinkEntry(archive, fh)

	case os.ModeNamedPipe, os.ModeSocket, os.ModeDevice:
		// Ignore the files that of these types
		logrus.Warningln("File ignored:", fileName)
		return nil

	default:
		return createZipFileEntry(archive, fh, level)
	}
}

func CreateZipArchive(w io.Writer, fileNames []string, level int) error {
	tracker := newPathErrorTracker()

	archive := zip.NewWriter(w)
	defer archive.Close()

	comp, err := flate.NewWriter(nil, level)
	if err != nil {
		return err
	}

	archive.RegisterCompressor(zip.Deflate, func(out io.Writer) (io.WriteCloser, error) {
		comp.Reset(out)

		return comp, nil
	})

	for _, fileName := range fileNames {
		if err := errorIfGitDirectory(fileName); tracker.actionable(err) {
			printGitArchiveWarning("archive")
		}

		err := createZipEntry(archive, fileName, level)
		if err != nil {
			return err
		}
	}

	return nil
}

func CreateZipFile(fileName string, fileNames []string, level int) error {
	// create directories to store archive
	err := os.MkdirAll(filepath.Dir(fileName), 0700)
	if err != nil {
		return err
	}

	tempFile, err := ioutil.TempFile(filepath.Dir(fileName), "archive_")
	if err != nil {
		return err
	}
	defer tempFile.Close()
	defer os.Remove(tempFile.Name())

	logrus.Debugln("Temporary file:", tempFile.Name())
	err = CreateZipArchive(tempFile, fileNames, level)
	if err != nil {
		return err
	}
	tempFile.Close()

	err = os.Rename(tempFile.Name(), fileName)
	if err != nil {
		return err
	}

	return nil
}
