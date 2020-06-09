package blocklinereplacer

import (
	"bytes"
	"fmt"
	"io"
	"strings"
)

type BlockLineReplacer struct {
	startLine string
	endLine   string

	replacement string
	output      *bytes.Buffer
	startFound  bool
	endFound    bool
}

func (r *BlockLineReplacer) Replace(inputContent string, replacement string) (string, error) {
	r.replacement = replacement
	r.output = new(bytes.Buffer)
	r.startFound = false
	r.endFound = false

	input := bytes.NewBufferString(inputContent)

	for {
		line, err := input.ReadString('\n')
		if err == io.EOF {
			break
		}

		if err != nil {
			return "", fmt.Errorf("reading input content: %w", err)
		}

		r.handleLine(line)
	}

	return r.output.String(), nil
}

func (r *BlockLineReplacer) handleLine(line string) {
	r.handleStart(line)
	r.handleRewrite(line)
	r.handleEnd(line)
}

func (r *BlockLineReplacer) handleStart(line string) {
	if r.startFound || !strings.Contains(line, r.startLine) {
		return
	}

	r.startFound = true
}

func (r *BlockLineReplacer) handleRewrite(line string) {
	if r.startFound && !r.endFound {
		return
	}

	r.output.WriteString(line)
}

func (r *BlockLineReplacer) handleEnd(line string) {
	if !strings.Contains(line, r.endLine) {
		return
	}

	r.endFound = true
	r.output.WriteString(r.replacement)
}

func New(startLine string, endLine string) *BlockLineReplacer {
	return &BlockLineReplacer{
		startLine: startLine,
		endLine:   endLine,
	}
}
