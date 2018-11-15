package shells

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"path"
	"path/filepath"
	"strings"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/helpers"
)

type Pwsh struct {
	AbstractShell
}

type PwshWriter struct {
	bytes.Buffer
	TemporaryPath string
	indent        int
}

func pwshQuote(text string) string {
	// taken from: http://www.robvanderwoude.com/escapechars.php
	text = strings.Replace(text, "`", "``", -1)
	// text = strings.Replace(text, "\0", "`0", -1)
	text = strings.Replace(text, "\a", "`a", -1)
	text = strings.Replace(text, "\b", "`b", -1)
	text = strings.Replace(text, "\f", "^f", -1)
	text = strings.Replace(text, "\r", "`r", -1)
	text = strings.Replace(text, "\n", "`n", -1)
	text = strings.Replace(text, "\t", "^t", -1)
	text = strings.Replace(text, "\v", "^v", -1)
	text = strings.Replace(text, "#", "`#", -1)
	text = strings.Replace(text, "'", "`'", -1)
	text = strings.Replace(text, "\"", "`\"", -1)
	return "\"" + text + "\""
}

func pwshQuoteVariable(text string) string {
	text = pwshQuote(text)
	text = strings.Replace(text, "$", "`$", -1)
	return text
}

func (b *PwshWriter) GetTemporaryPath() string {
	return b.TemporaryPath
}

func (b *PwshWriter) Line(text string) {
	b.WriteString(strings.Repeat("  ", b.indent) + text + "\r\n")
}

func (b *PwshWriter) CheckForErrors() {
	b.checkErrorLevel()
}

func (b *PwshWriter) Indent() {
	b.indent++
}

func (b *PwshWriter) Unindent() {
	b.indent--
}

func (b *PwshWriter) checkErrorLevel() {
	b.Line("if(!$?) { Exit $LASTEXITCODE }")
	b.Line("")
}

func (b *PwshWriter) Command(command string, arguments ...string) {
	b.Line(b.buildCommand(command, arguments...))
	b.checkErrorLevel()
}

func (b *PwshWriter) buildCommand(command string, arguments ...string) string {
	list := []string{
		pwshQuote(command),
	}

	for _, argument := range arguments {
		list = append(list, pwshQuote(argument))
	}

	return "& " + strings.Join(list, " ")
}

func (b *PwshWriter) TmpFile(name string) string {
	filePath := b.Absolute(path.Join(b.TemporaryPath, name))
	return helpers.ToBackslash(filePath)
}

func (b *PwshWriter) EnvVariableKey(name string) string {
	return fmt.Sprintf("$%s", name)
}

func (b *PwshWriter) Variable(variable common.JobVariable) {
	if variable.File {
		variableFile := b.TmpFile(variable.Key)
		b.Line(fmt.Sprintf("md %s -Force | out-null", pwshQuote(helpers.ToBackslash(b.TemporaryPath))))
		b.Line(fmt.Sprintf("Set-Content %s -Value %s -Encoding UTF8 -Force", pwshQuote(variableFile), pwshQuoteVariable(variable.Value)))
		b.Line("$" + variable.Key + "=" + pwshQuote(variableFile))
	} else {
		b.Line("$" + variable.Key + "=" + pwshQuoteVariable(variable.Value))
	}

	b.Line("$env:" + variable.Key + "=$" + variable.Key)
}

func (b *PwshWriter) IfDirectory(path string) {
	b.Line("if(Test-Path " + pwshQuote(helpers.ToBackslash(path)) + " -PathType Container) {")
	b.Indent()
}

func (b *PwshWriter) IfFile(path string) {
	b.Line("if(Test-Path " + pwshQuote(helpers.ToBackslash(path)) + " -PathType Leaf) {")
	b.Indent()
}

func (b *PwshWriter) IfCmd(cmd string, arguments ...string) {
	b.Line(b.buildCommand(cmd, arguments...) + " 2>$null")
	b.Line("if($?) {")
	b.Indent()
}

func (b *PwshWriter) IfCmdWithOutput(cmd string, arguments ...string) {
	b.Line(b.buildCommand(cmd, arguments...))
	b.Line("if($?) {")
	b.Indent()
}

func (b *PwshWriter) Else() {
	b.Unindent()
	b.Line("} else {")
	b.Indent()
}

func (b *PwshWriter) EndIf() {
	b.Unindent()
	b.Line("}")
}

func (b *PwshWriter) Cd(path string) {
	b.Line("cd " + pwshQuote(helpers.ToBackslash(path)))
	b.checkErrorLevel()
}

func (b *PwshWriter) MkDir(path string) {
	b.Line(fmt.Sprintf("md %s -Force | out-null", pwshQuote(helpers.ToBackslash(path))))
}

func (b *PwshWriter) MkTmpDir(name string) string {
	path := helpers.ToBackslash(path.Join(b.TemporaryPath, name))
	b.MkDir(path)

	return path
}

func (b *PwshWriter) RmDir(path string) {
	path = pwshQuote(helpers.ToBackslash(path))
	b.Line("if( (Get-Command -Name Remove-Item2 -Module NTFSSecurity -ErrorAction SilentlyContinue) -and (Test-Path " + path + " -PathType Container) ) {")
	b.Indent()
	b.Line("Remove-Item2 -Force -Recurse " + path)
	b.Unindent()
	b.Line("} elseif(Test-Path " + path + ") {")
	b.Indent()
	b.Line("Remove-Item -Force -Recurse " + path)
	b.Unindent()
	b.Line("}")
	b.Line("")
}

func (b *PwshWriter) RmFile(path string) {
	path = pwshQuote(helpers.ToBackslash(path))
	b.Line("if( (Get-Command -Name Remove-Item2 -Module NTFSSecurity -ErrorAction SilentlyContinue) -and (Test-Path " + path + " -PathType Leaf) ) {")
	b.Indent()
	b.Line("Remove-Item2 -Force " + path)
	b.Unindent()
	b.Line("} elseif(Test-Path " + path + ") {")
	b.Indent()
	b.Line("Remove-Item -Force " + path)
	b.Unindent()
	b.Line("}")
	b.Line("")
}

func (b *PwshWriter) Print(format string, arguments ...interface{}) {
	coloredText := helpers.ANSI_RESET + fmt.Sprintf(format, arguments...)
	b.Line("echo " + pwshQuoteVariable(coloredText))
}

func (b *PwshWriter) Notice(format string, arguments ...interface{}) {
	coloredText := helpers.ANSI_BOLD_GREEN + fmt.Sprintf(format, arguments...) + helpers.ANSI_RESET
	b.Line("echo " + pwshQuoteVariable(coloredText))
}

func (b *PwshWriter) Warning(format string, arguments ...interface{}) {
	coloredText := helpers.ANSI_YELLOW + fmt.Sprintf(format, arguments...) + helpers.ANSI_RESET
	b.Line("echo " + pwshQuoteVariable(coloredText))
}

func (b *PwshWriter) Error(format string, arguments ...interface{}) {
	coloredText := helpers.ANSI_BOLD_RED + fmt.Sprintf(format, arguments...) + helpers.ANSI_RESET
	b.Line("echo " + pwshQuoteVariable(coloredText))
}

func (b *PwshWriter) EmptyLine() {
	b.Line("echo \"\"")
}

func (b *PwshWriter) Absolute(dir string) string {
	if filepath.IsAbs(dir) {
		return dir
	}

	b.Line("$CurrentDirectory = (Resolve-Path .\\).Path")
	return filepath.Join("$CurrentDirectory", dir)
}

func (b *PwshWriter) Finish(trace bool) string {
	var buffer bytes.Buffer
	w := bufio.NewWriter(&buffer)

	// write BOM
	io.WriteString(w, "\xef\xbb\xbf")
	if trace {
		io.WriteString(w, "Set-PSDebug -Trace 2\r\n")
	}

	io.WriteString(w, b.String())
	w.Flush()
	return buffer.String()
}

func (b *Pwsh) GetName() string {
	return "pwsh"
}

func (b *Pwsh) GetConfiguration(info common.ShellScriptInfo) (script *common.ShellConfiguration, err error) {
	script = &common.ShellConfiguration{
		Command:   "pwsh",
		Arguments: []string{"-noprofile", "-noninteractive", "-executionpolicy", "Bypass", "-command"},
		PassFile:  true,
		Extension: "ps1",
	}
	return
}

func (b *Pwsh) GenerateScript(buildStage common.BuildStage, info common.ShellScriptInfo) (script string, err error) {
	w := &PwshWriter{
		TemporaryPath: info.Build.FullProjectDir() + ".tmp",
	}

	if buildStage == common.BuildStagePrepare {
		if len(info.Build.Hostname) != 0 {
			w.Line("echo \"Running on $env:computername via " + pwshQuoteVariable(info.Build.Hostname) + "...\"")
		} else {
			w.Line("echo \"Running on $env:computername...\"")
		}
	}

	err = b.writeScript(w, buildStage, info)
	script = w.Finish(info.Build.IsDebugTraceEnabled())
	return
}

func (b *Pwsh) IsDefault() bool {
	return false
}

func init() {
	common.RegisterShell(&Pwsh{})
}
