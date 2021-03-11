package runnershell

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	"os/exec"
)

// An Action represents one function to be executed. Whilst an Action
// struct contains entries for all supported functions, this is to ease
// json serialization, where each field is used as a function discriminator.
//
// Only one function within an action should be set, setting more is undefined
// behavior.
type (
	Function interface {
		execute(*Options) error
	}

	Action struct {
		Line  *Line  `json:"line,omitempty"`
		Cd    *Cd    `json:"cd,omitempty"`
		MkDir *MkDir `json:"mkdir,omitempty"`
		Rm    *Rm    `json:"rm,omitempty"`
		Print *Print `json:"print,omitempty"`
		Cmd   *Cmd   `json:"cmd,omitempty"`
		If    *If    `json:"if,omitempty"`
	}

	Actions []*Action

	Line  string
	Cd    string
	MkDir string
	Rm    string
	Print string

	Cmd struct {
		Arguments []string `json:"args,omitempty"`
		Output    bool     `json:"output,omitempty"`
	}

	If struct {
		Directory string  `json:"directory,omitempty"`
		File      string  `json:"file,omitempty"`
		Cmd       Cmd     `json:"cmd,omitempty"`
		Actions   Actions `json:"actions,omitempty"`
		Else      Actions `json:"else,omitempty"`
	}

	Options struct {
		Trace        bool
		ShellCommand []string

		Stdout io.Writer
		Stderr io.Writer
		Env    []string

		subshell      *exec.Cmd
		subshellStdin io.WriteCloser
	}
)

func (s Script) Execute(options Options) error {
	if options.Stdout == nil {
		options.Stdout = os.Stdout
	}
	if options.Stderr == nil {
		options.Stderr = os.Stderr
	}

	// extract variables
	for _, v := range s.Variables {
		if v.File {
			h := sha256.New()
			_, _ = h.Write([]byte(v.Key))
			_, _ = h.Write([]byte(v.Value))
			key := h.Sum(nil)

			filepath := filepath.Join(s.TemporaryPath, hex.EncodeToString(key))

			// don't write the file if it already exists
			_, err := os.Stat(filepath)
			if err != nil {
				if err := ioutil.WriteFile(filepath, []byte(v.Value), 0777); err != nil {
					return fmt.Errorf("could not write file variable %s: %w", v.Key, err)
				}
			}

			options.Env = append(options.Env, v.Key+"="+filepath)
		} else {
			options.Env = append(options.Env, fmt.Sprintf("%s=%s", v.Key, v.Value))
		}
	}

	// execute script
	return s.Actions.execute(&options)
}

func (a Actions) execute(options *Options) error {
	for _, action := range a {
		var fn Function

		switch {
		case action.Line != nil:
			fn = action.Line
		case action.Cd != nil:
			fn = action.Cd
		case action.MkDir != nil:
			fn = action.MkDir
		case action.Rm != nil:
			fn = action.Rm
		case action.Print != nil:
			fn = action.Print
		case action.Cmd != nil:
			fn = action.Cmd
		case action.If != nil:
			fn = action.If
		}

		if fn == nil {
			return fmt.Errorf("unknown action function")
		}

		if err := fn.execute(options); err != nil {
			return err
		}
	}

	if options.subshell != nil {
		options.subshellStdin.Close()

		defer func() {
			options.subshell = nil
		}()

		return options.subshell.Wait()
	}

	return nil
}

func (expr Cd) execute(options *Options) error {
	if options.Trace {
		fmt.Fprintln(options.Stderr, "> Changing directory: ", expr)
	}

	return os.Chdir(string(expr))
}

func (expr MkDir) execute(options *Options) error {
	if options.Trace {
		fmt.Fprintln(options.Stderr, "> mkdir: ", expr)
	}

	return os.MkdirAll(string(expr), 0777)
}

func (expr Rm) execute(options *Options) error {
	if options.Trace {
		fmt.Fprintln(options.Stderr, "> rm: ", expr)
	}

	return os.RemoveAll(string(expr))
}

func (expr Print) execute(options *Options) error {
	if options.Trace {
		fmt.Fprintln(options.Stderr, "> print: ", expr)
	}

	_, err := fmt.Fprintln(options.Stdout, string(expr))
	return err
}

func (expr Line) execute(options *Options) error {
	if len(options.ShellCommand) == 0 {
		return fmt.Errorf("no subshell command options have been provided")
	}

	if options.subshell == nil {
		options.subshell = exec.Command(options.ShellCommand[0], options.ShellCommand[1:]...)
		options.subshell.Env = options.Env
		options.subshell.Stdout = options.Stdout
		options.subshell.Stderr = options.Stderr

		stdin, err := options.subshell.StdinPipe()
		if err != nil {
			return err
		}

		if err := options.subshell.Start(); err != nil {
			return err
		}

		options.subshellStdin = stdin
	}

	if options.Trace {
		fmt.Fprintln(options.Stdout, ">>", expr)
	}

	_, err := options.subshellStdin.Write([]byte(expr + "\n"))
	time.Sleep(10 * time.Millisecond)

	return err
}

func (expr Cmd) execute(options *Options) error {
	if options.Trace {
		fmt.Fprintln(options.Stdout, ">", strings.Join(expr.Arguments, " "))
	}

	if len(expr.Arguments) > 0 {
		cmd := exec.Command(expr.Arguments[0], expr.Arguments[1:]...)
		cmd.Env = options.Env

		if expr.Output {
			cmd.Stdout = options.Stdout
			cmd.Stderr = options.Stderr
		}

		return cmd.Run()
	}

	return nil
}

func (expr If) execute(options *Options) error {
	var passed bool

	switch {
	case expr.File != "":
		fi, err := os.Stat(expr.File)
		passed = err == nil && !fi.IsDir()

	case expr.Directory != "":
		fi, err := os.Stat(expr.Directory)
		passed = err == nil && fi.IsDir()

	case len(expr.Cmd.Arguments) > 0:
		passed = expr.Cmd.execute(options) == nil
	}

	if passed {
		return expr.Actions.execute(options)
	}

	if len(expr.Else) > 0 {
		return expr.Else.execute(options)
	}

	return fmt.Errorf("if statement failed")
}
