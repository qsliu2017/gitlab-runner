package cli_helpers

import (
	"syscall"

	"github.com/konsorten/go-windows-terminal-sequences"
)

// InitCli initializes the Windows console window by activating virtual terminal features.
// Calling this function enables colored terminal output.
func InitCli() {
	sequences.EnableVirtualTerminalProcessing(syscall.Stdout, true) // enable VT processing on standard output stream
	sequences.EnableVirtualTerminalProcessing(syscall.Stderr, true) // enable VT processing on standard error stream
}
