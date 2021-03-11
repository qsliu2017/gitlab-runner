package runnershell

import (
	"encoding/json"
	"fmt"
	"path"
	"strings"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/helpers"
)

type Script struct {
	TemporaryPath string               `json:"temporary-path,omitempty"`
	Trace         bool                 `json:"trace,omitempty"`
	ShellCommand  []string             `json:"shell-command,omitempty"`
	Variables     []common.JobVariable `json:"variables,omitempty"`
	Actions       Actions              `json:"actions,omitempty"`
}

type Writer struct {
	Script Script

	stack []*Actions
	elses []*Actions
}

func (w *Writer) add(fn Function) {
	if len(w.stack) == 0 {
		w.stack = append(w.stack, &w.Script.Actions)
	}

	action := new(Action)

	switch f := fn.(type) {
	case Line:
		action.Line = &f
	case Cd:
		action.Cd = &f
	case MkDir:
		action.MkDir = &f
	case Rm:
		action.Rm = &f
	case Print:
		action.Print = &f
	case Cmd:
		action.Cmd = &f
	case If:
		action.If = &f
	}

	top := len(w.stack) - 1
	*w.stack[top] = append(*w.stack[top], action)

	if _, ok := fn.(If); ok {
		w.stack = append(w.stack, &action.If.Actions)
		w.elses = append(w.elses, &action.If.Else)
	}
}

func (w *Writer) EnvVariableKey(name string) string {
	return name
}

func (w *Writer) Variable(variable common.JobVariable) {
	w.Script.Variables = append(w.Script.Variables, variable)
}

func (w *Writer) Command(command string, arguments ...string) {
	w.add(Cmd{
		Arguments: append([]string{command}, arguments...),
		Output:    true,
	})
}

func (w *Writer) Line(text string) {
	w.add(Line(text))
}

func (w *Writer) CheckForErrors() {

}

func (w *Writer) IfDirectory(name string) {
	w.add(If{Directory: name})
}

func (w *Writer) IfFile(name string) {
	w.add(If{File: name})
}

func (w *Writer) IfCmd(cmd string, arguments ...string) {
	w.add(If{Cmd: Cmd{Arguments: append([]string{cmd}, arguments...)}})
}

func (w *Writer) IfCmdWithOutput(cmd string, arguments ...string) {
	w.add(If{Cmd: Cmd{Arguments: append([]string{cmd}, arguments...), Output: true}})
}

func (w *Writer) Else() {
	w.stack[len(w.stack)-1] = w.elses[len(w.elses)-1]
}

func (w *Writer) EndIf() {
	w.stack[len(w.stack)-1] = nil
	w.elses[len(w.elses)-1] = nil

	w.stack = w.stack[:len(w.stack)-1]
	w.elses = w.elses[:len(w.elses)-1]

	if len(w.stack) == 0 {
		panic("closing if with none open")
	}
}

func (w *Writer) Cd(name string) {
	w.add(Cd(name))
}

func (w *Writer) MkDir(name string) {
	w.add(MkDir(name))
}

func (w *Writer) RmDir(name string) {
	w.add(Rm(name))
}

func (w *Writer) RmFile(name string) {
	w.add(Rm(name))
}

func (w *Writer) Join(elem ...string) string {
	return path.Join(elem...)
}

func (w *Writer) TmpFile(name string) string {
	return path.Join(w.Script.TemporaryPath, name)
}

func (w *Writer) MkTmpDir(name string) string {
	name = path.Join(w.Script.TemporaryPath, name)
	w.MkDir(name)
	return name
}

func (w *Writer) Printf(format string, arguments ...interface{}) {
	w.print(helpers.ANSI_RESET, format, arguments)
}

func (w *Writer) Noticef(format string, arguments ...interface{}) {
	w.print(helpers.ANSI_BOLD_GREEN, format, arguments)
}

func (w *Writer) Warningf(format string, arguments ...interface{}) {
	w.print(helpers.ANSI_YELLOW, format, arguments)
}

func (w *Writer) Errorf(format string, arguments ...interface{}) {
	w.print(helpers.ANSI_BOLD_RED, format, arguments)
}

func (w *Writer) print(color string, format string, arguments []interface{}) {
	w.add(Print(color + fmt.Sprintf(format, arguments...) + helpers.ANSI_RESET))
}

func (w *Writer) EmptyLine() {

}

func (w *Writer) Finish(trace bool) string {
	if len(w.stack) > 1 {
		panic("script finished with open if statements")
	}

	w.Script.Trace = trace

	var result strings.Builder

	enc := json.NewEncoder(&result)
	enc.SetIndent("", "\t")
	err := enc.Encode(w.Script)

	if err != nil {
		// MustEncode: we panic here, as the only possible error is our struct
		// contains fields that are unable to be processed by the json encoder.
		panic(err)
	}

	return result.String()
}
