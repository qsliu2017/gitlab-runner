package trace

import (
	"fmt"
	"hash"
	"hash/crc32"
	"io"
	"io/ioutil"
	"os"
	"sync"

	"gitlab.com/gitlab-org/gitlab-runner/helpers"
	"golang.org/x/text/transform"
)

type Trace struct {
	f        *os.File
	lw       *limitWriter
	w        io.WriteCloser
	limit    int
	checksum hash.Hash32
	lock     sync.Mutex

	transformers []transform.Transformer
}

func New() (*Trace, error) {
	f, err := ioutil.TempFile("", "trace")
	if err != nil {
		return nil, err
	}

	buffer := &Trace{
		f:        f,
		limit:    4 * 1024 * 1024, // 4MB
		checksum: crc32.NewIEEE(),
	}

	return buffer, nil
}

func (b *Trace) Write(p []byte) (int, error) {
	b.lock.Lock()
	defer b.lock.Unlock()

	if b.w == nil {
		b.lw = &limitWriter{
			w:       io.MultiWriter(b.f, b.checksum),
			written: 0,
			limit:   int64(b.limit),
		}

		b.w = transform.NewWriter(b.lw, transform.Chain(b.transformers...))
	}

	n, err := b.w.Write(p)
	if err == io.ErrShortWrite {
		return n, nil
	}
	return n, err
}

func (b *Trace) Bytes(offset, n int) ([]byte, error) {
	return ioutil.ReadAll(io.NewSectionReader(b.f, int64(offset), int64(n)))
}

func (b *Trace) Finish() {
	if b.w != nil {
		b.w.Close()
	}
}

func (b *Trace) Close() {
	_ = b.f.Close()
	_ = os.Remove(b.f.Name())
}

func (b *Trace) SetLimit(size int) {
	b.limit = size
}

func (b *Trace) Size() int {
	if b.lw == nil {
		return 0
	}
	return int(b.lw.written)
}

func (b *Trace) Checksum() string {
	return fmt.Sprintf("crc32:%08x", b.checksum.Sum32())
}

func (b *Trace) SetMasked(values []string) {
	b.transformers = make([]transform.Transformer, 0, len(values)+1)

	for _, value := range values {
		b.transformers = append(b.transformers, NewPhraseTransform(value))
	}

	b.transformers = append(b.transformers, NewSensitiveURLParamTransform())
}

type limitWriter struct {
	w       io.Writer
	written int64
	limit   int64
}

func (w *limitWriter) Write(p []byte) (int, error) {
	capacity := w.limit - w.written

	if w.written >= w.limit {
		return 0, io.ErrShortWrite
	}

	if int64(len(p)) >= capacity {
		p = p[:capacity]
		n, err := w.w.Write(p)
		if err == nil {
			err = io.ErrShortWrite
		}
		w.written += int64(n)
		w.writeLimitExceededMessage()

		return n, err
	}

	n, err := w.w.Write(p)
	w.written += int64(n)
	return n, err
}

func (w *limitWriter) writeLimitExceededMessage() {
	msg := "\n%sJob's log exceeded limit of %v bytes.%s\n"
	n, _ := fmt.Fprintf(w.w, msg, helpers.ANSI_BOLD_RED, w.limit, helpers.ANSI_RESET)
	w.written += int64(n)
}
