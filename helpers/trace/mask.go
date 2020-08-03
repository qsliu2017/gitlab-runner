package trace

import (
	"bytes"

	"golang.org/x/text/transform"
)

const Mask = "[MASKED]"

// NewPhaseTransform returns a transform.Transformer that replaces the phrase
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

			n := copy(dst[nDst:], src[nSrc:nSrc+i])
			nDst += n
			nSrc += n
			if n < i {
				return nDst, nSrc, transform.ErrShortDst
			}
		}

		// replace phrase
		nDst, nSrc, err = replace(dst, nDst, nSrc, []byte(Mask), len(t))
		if err != nil {
			return nDst, nSrc, err
		}
	}

	return safecopy(src, dst, atEOF, nSrc, nDst, len(t))
}

// NewURLParamTransform returns a transform.Transformer that replaces common
// sensitive param values with [MASKED]
func NewSensitiveURLParamTransform() transform.Transformer {
	return sensitiveURLParamTransform{}
}

type sensitiveURLParamTransform struct {
}

func (sensitiveURLParamTransform) Reset() {}

func (t sensitiveURLParamTransform) hasParam(query []byte) bool {
	params := [][]byte{
		[]byte("private_token"),
		[]byte("authenticity_token"),
		[]byte("rss_token"),
		[]byte("x-amz-signature"),
		[]byte("x-amz-credential"),
	}

	for _, param := range params {
		if bytes.EqualFold(query, param) {
			return true
		}
	}
	return false
}

//nolint:gocognit
func (t sensitiveURLParamTransform) Transform(dst, src []byte, atEOF bool) (nDst, nSrc int, err error) {
	// To prevent missing tokens that appear on a boundary, we need to ensure
	// we have sufficient data. Tokens can be quite long.
	const maxTokenSize = 255

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
			end := bytes.IndexAny(src[nSrc+idx:], "& ")
			if end == -1 {
				if !atEOF {
					break
				}

				// if we're atEOF, everything is the value
				end = len(src[nSrc+idx:])
			}

			// copy everything up until the value
			off := len(src[nSrc : nSrc+idx])
			n := copy(dst[nDst:], src[nSrc:nSrc+idx])
			nDst += n
			nSrc += n
			if n < off {
				return nDst, nSrc, transform.ErrShortDst
			}

			value := src[nSrc : nSrc+end]
			if t.hasParam(key[1:]) {
				value = []byte("=" + Mask)
			}

			nDst, nSrc, err = replace(dst, nDst, nSrc, value, end)
			if err != nil {
				return nDst, nSrc, err
			}
		}
	}

	return safecopy(src, dst, atEOF, nSrc, nDst, maxTokenSize)
}

// replace copies a replacement into the dst buffer and advances the nSrc.
func replace(dst []byte, nDst, nSrc int, replacement []byte, advance int) (int, int, error) {
	n := copy(dst[nDst:], replacement)
	if n < len(replacement) {
		return nDst + n, nSrc, transform.ErrShortDst
	}

	return nDst + n, nSrc + advance, nil
}

// safecopy copies the remaining data minus that of the token size, preventing
// the accidental copy of the beginning of a token that should be replaced. If
// atEOF is true, the full remaining data is copied.
func safecopy(src, dst []byte, atEOF bool, nSrc, nDst int, tokenSize int) (int, int, error) {
	var err error

	remaining := len(src[nSrc:])
	if !atEOF {
		remaining -= tokenSize + 1
		err = transform.ErrShortSrc
	}

	if remaining > 0 {
		n := copy(dst[nDst:], src[nSrc:nSrc+remaining])
		nDst += n
		nSrc += n
		if n < remaining {
			return nDst, nSrc, transform.ErrShortDst
		}
	}

	return nDst, nSrc, err
}
