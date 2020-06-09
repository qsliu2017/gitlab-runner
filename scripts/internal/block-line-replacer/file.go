package blocklinereplacer

import (
	"fmt"
	"io/ioutil"
)

type FileReplacer struct {
	filePath string
	replacer *BlockLineReplacer

	fileContent string
	replacement string
}

func (fr *FileReplacer) Replace(replacement string) error {
	fr.replacement = replacement

	for _, f := range []func() error{
		fr.getFileContent,
		fr.replace,
		fr.saveFileContent,
	} {
		err := f()
		if err != nil {
			return err
		}
	}

	return nil
}

func (fr *FileReplacer) getFileContent() error {
	data, err := ioutil.ReadFile(fr.filePath)
	if err != nil {
		return fmt.Errorf("reading file %q: %w", fr.filePath, err)
	}

	fr.fileContent = string(data)

	return nil
}

func (fr *FileReplacer) replace() error {
	var err error

	fr.fileContent, err = fr.replacer.Replace(fr.fileContent, fr.replacement)
	if err != nil {
		return fmt.Errorf("replacing content: %w", err)
	}

	return nil
}

func (fr *FileReplacer) saveFileContent() error {
	err := ioutil.WriteFile(fr.filePath, []byte(fr.fileContent), 0644)
	if err != nil {
		return fmt.Errorf(" writing new content for file %q: %w", fr.filePath, err)
	}

	return nil
}

func NewFileReplacer(filePath string, replacer *BlockLineReplacer) *FileReplacer {
	return &FileReplacer{
		filePath: filePath,
		replacer: replacer,
	}
}
