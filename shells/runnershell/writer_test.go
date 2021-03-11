package runnershell

import (
	"io/ioutil"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestWriterBranches(t *testing.T) {
	w := Writer{}

	//nolint:gocritic
	// ironically, using nolint:gocritic here is to silence "unneccessaryBlocks", and yet
	// to silence this whole section, I have to add an outer block for the linter.
	{
		w.IfCmd("if command", "is", "successful")
		{
			w.Cd("then cd into this directory")

			w.IfDirectory("and if this directory exists")
			{
				w.IfCmdWithOutput("run another command and display the output")
				{
					w.Printf("that command with output succeeded")
				}
				w.Else()
				{
					w.Printf("that command with output failed")
				}
				w.EndIf()

				w.Noticef("removing stuff...")

				w.IfFile("if it has a file")
				{
					w.RmFile("remove a file")
				}
				w.EndIf()

				w.RmDir("delete the directory")
			}
			w.Else()
			{
				w.Command("run a command")
				w.MkDir("maybe make directory")
				w.Errorf("and this else statement has multiple actions")
			}
			w.EndIf()

			w.Warningf("we've finished")

			w.Line("echo run a script line")
			w.Line("echo inside a subshell")
		}
		w.EndIf()
	}

	branches, err := ioutil.ReadFile("testdata/branches.json")
	require.NoError(t, err)
	require.Equal(t, w.Finish(true), string(branches))
}
