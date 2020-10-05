package trace

import (
	"bytes"

	"golang.org/x/text/transform"
)

const (
	// Mask is the string that replaces any found sensitive information
	Mask = "[MASKED]"

	// sensitiveURLMaxTokenSize is the max token size we consider for sensitive
	// URL param values. This prevents missing tokens that appear on a boundary
	// and ensures we always have sufficient data. Param values can get quite
	// long.
	sensitiveURLMaxTokenSize = 255
)

var (
	// sensitiveURLTokens are the param tokens we search for and replace the
	// values of to [MASKED].
	sensitiveURLTokens = [][]byte{
		[]byte("private_token"),
		[]byte("authenticity_token"),
		[]byte("rss_token"),
		[]byte("x-amz-signature"),
		[]byte("x-amz-credential"),
	}
)

// NewPhaseTransform returns a transform.Transformer that replaces the `phrase`
// with [MASKED]
func NewPhraseTransform(phrase string) transform.Transformer {
	return phraseTransform([]byte(phrase))
}

type phraseTransform []byte

func (phraseTransform) Reset() {}

func (t phraseTransform) Transform(dst, src []byte, atEOF bool) (nDst, nSrc int, err error) {
	for {
		// copy up until phrase
		{
			i := bytes.Index(src[nSrc:], t)
			if i == -1 {
				break
			}

			err := copyn(dst, src, &nDst, &nSrc, i)
			if err != nil {
				return nDst, nSrc, err
			}
		}

		// replace phrase
		err = replace(dst, &nDst, &nSrc, []byte(Mask), len(t))
		if err != nil {
			return nDst, nSrc, err
		}
	}

	return safecopy(dst, src, atEOF, nDst, nSrc, len(t))
}

// NewSensitiveURLParamTransform returns a transform.Transformer that replaces common
// sensitive param values with [MASKED]
func NewSensitiveURLParamTransform() transform.Transformer {
	return sensitiveURLParamTransform(sensitiveURLTokens)
}

type sensitiveURLParamTransform [][]byte

func (sensitiveURLParamTransform) Reset() {}

func (t sensitiveURLParamTransform) hasSensitiveParam(query []byte) bool {
	for _, param := range t {
		if bytes.EqualFold(query, param) {
			return true
		}
	}
	return false
}

//nolint:gocognit
func (t sensitiveURLParamTransform) Transform(dst, src []byte, atEOF bool) (nDst, nSrc int, err error) {
	for {
		// copy up until ? or &, the start of a url parameter
		idx := bytes.IndexAny(src[nSrc:], "?&")
		if idx == -1 {
			break
		}

		// key
		var key []byte
		{
			keyIndex := bytes.IndexAny(src[nSrc+idx:], "=")
			if keyIndex == -1 {
				break
			}

			key = src[nSrc+idx : nSrc+idx+keyIndex]
			idx += keyIndex
		}

		// replace value
		{
			end := t.indexOfParamEnd(src[nSrc+idx:], atEOF)
			if end == -1 {
				break
			}

			// copy everything up until the value
			err := copyn(dst, src, &nDst, &nSrc, idx)
			if err != nil {
				return nDst, nSrc, err
			}

			value := src[nSrc : nSrc+end]
			if t.hasSensitiveParam(key[1:]) {
				value = []byte("=" + Mask)
			}

			err = replace(dst, &nDst, &nSrc, value, end)
			if err != nil {
				return nDst, nSrc, err
			}
		}
	}

	return safecopy(dst, src, atEOF, nDst, nSrc, sensitiveURLMaxTokenSize)
}

func (t sensitiveURLParamTransform) indexOfParamEnd(src []byte, atEOF bool) int {
	end := bytes.IndexAny(src, "& ")
	if end == -1 {
		if !atEOF {
			return -1
		}

		// if we're atEOF, everything is the value
		end = len(src)
	}

	return end
}

// replace copies a replacement into the dst buffer and advances nDst and nSrc.
func replace(dst []byte, nDst, nSrc *int, replacement []byte, advance int) error {
	n := copy(dst[*nDst:], replacement)
	*nDst += n
	if n < len(replacement) {
		return transform.ErrShortDst
	}
	*nSrc += advance

	return nil
}

// copy copies data from src to dst for length n and advances nDst and nSrc.
func copyn(dst, src []byte, nDst, nSrc *int, n int) error {
	copied := copy(dst[*nDst:], src[*nSrc:*nSrc+n])
	*nDst += copied
	*nSrc += copied
	if copied < n {
		return transform.ErrShortDst
	}

	return nil
}

// safecopy copies the remaining data minus that of the token size, preventing
// the accidental copy of the beginning of a token that should be replaced. If
// atEOF is true, the full remaining data is copied.
func safecopy(dst, src []byte, atEOF bool, nDst, nSrc int, tokenSize int) (int, int, error) {
	var err error

	remaining := len(src[nSrc:])
	if !atEOF {
		remaining -= tokenSize + 1
		err = transform.ErrShortSrc
	}

	if remaining > 0 {
		err := copyn(dst, src, &nDst, &nSrc, remaining)
		if err != nil {
			return nDst, nSrc, err
		}
	}

	return nDst, nSrc, err
}
