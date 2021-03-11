package runnershell

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCommandExecution(t *testing.T) {
	w := Writer{}

	dir, err := ioutil.TempDir("", "")
	require.NoError(t, err)

	defer os.RemoveAll(dir)

	// executing the below script changes the directory,
	// so we need to restore it after the test.
	currentDir, _ := os.Getwd()
	defer func() {
		_ = os.Chdir(currentDir)
	}()

	w.IfDirectory(dir)
	w.Cd(dir)
	w.Printf("We're not in the temporary directory")
	w.EndIf()

	w.MkDir("subdir")
	w.IfDirectory("subdir")
	w.Cd("subdir")
	w.EndIf()

	w.IfDirectory("nested")
	w.Noticef("nested should not exist")
	w.Else()
	w.Noticef("nested correctly doesn't exist")
	w.EndIf()

	w.Command("echo", "hello world")
	w.RmDir("subdir")

	err = w.Script.Execute(Options{})
	require.NoError(t, err)
}
