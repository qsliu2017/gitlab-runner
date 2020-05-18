package jobcontrol

import (
	"context"
	"io"
	"os/exec"
	"time"
)

// DefaultKillAttempts is the number of soft kill attempts before a hard
// kill.
const DefaultKillAttempts = 1

// DefaultKillTimeout is the delay between soft kill attempts and the final
// hard kill.
const DefaultKillTimeout = 5 * time.Second

// JobCmd represents an external command being prepared or run.
//
// It differs from os/exec.Cmd in that when the provided context is cancelled,
// the process and all of its children exit.
type JobCmd struct {
	cmd *exec.Cmd
	ctx context.Context

	name   string
	Args   []string
	Env    []string
	Dir    string
	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer

	KillAttempts int           // attempts to soft kill before a hard kill
	KillTimeout  time.Duration // delay between kills

	// used on windows only
	jobObjectHandle uintptr
}

// JobCommand returns the JobCmd struct to execute the named program with the
// given arguments.
func Command(ctx context.Context, name string, arg ...string) *JobCmd {
	return &JobCmd{
		ctx:          ctx,
		name:         name,
		Args:         append([]string{name}, arg...),
		KillTimeout:  DefaultKillTimeout,
		KillAttempts: DefaultKillAttempts,
	}
}

// Start starts the specified command but does not wait for it to complete.
//
// It is identical to os/exec.Cmd.Start with the following differences:
// - For unix-like operating systems, the process gets a new process group id.
// - For windows, the process is assigned to a job object.
// - When the process exits, so do any children it spawned.
func (c *JobCmd) Start() error {
	c.cmd = exec.Command(c.name)
	c.cmd.Args = c.Args
	c.cmd.Env = c.Env
	c.cmd.Dir = c.Dir
	c.cmd.Stdin = c.Stdin
	c.cmd.Stdout = c.Stdout
	c.cmd.Stderr = c.Stderr

	return c.start()
}

// Wait waits for the command to exit and waits for any copying to stdin or
// copying from stdout or stderr to complete.
//
// The command must have been started by Start.
//
// If the context supplied to `Command` is cancelled, a graceful kill is
// attempted followed by complete termination.
func (c *JobCmd) Wait() error {
	waitCh := make(chan error)
	go func() {
		waitCh <- c.cmd.Wait()
	}()
	defer c.hardKill()

	killCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	select {
	case err := <-waitCh:
		return err

	case <-c.ctx.Done():
		go c.waitKill(killCtx)
	}

	return <-waitCh
}

func (c *JobCmd) waitKill(ctx context.Context) {
	defer c.hardKill()

	ticker := time.NewTicker(c.KillTimeout)
	defer ticker.Stop()

	for attempt := 0; attempt < c.KillAttempts; attempt++ {
		c.softKill()

		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// waits between soft kills / before final kill
		}
	}
}
