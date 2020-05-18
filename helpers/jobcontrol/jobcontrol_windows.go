package jobcontrol

import (
	"fmt"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

type process struct {
	Pid    int
	Handle uintptr
}

func (c *JobCmd) start() error {
	c.cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP,
	}

	err := c.cmd.Start()
	if err != nil {
		return err
	}

	if err := c.createJobObject(); err != nil {
		return fmt.Errorf("creating job object: %w", err)
	}

	err = windows.AssignProcessToJobObject(
		windows.Handle(c.jobObjectHandle),
		windows.Handle((*process)(unsafe.Pointer(c.cmd.Process)).Handle))
	if err != nil {
		return fmt.Errorf("assigning process to job object: %w", err)
	}

	return nil
}

func (c *JobCmd) softKill() {
	_ = windows.GenerateConsoleCtrlEvent(windows.CTRL_BREAK_EVENT, uint32(c.cmd.Process.Pid))
}

func (c *JobCmd) hardKill() {
	_ = windows.CloseHandle(windows.Handle(c.jobObjectHandle))
}

func (c *JobCmd) createJobObject() error {
	handle, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return err
	}
	c.jobObjectHandle = uintptr(handle)

	info := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{
		BasicLimitInformation: windows.JOBOBJECT_BASIC_LIMIT_INFORMATION{
			LimitFlags: windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE,
		},
	}

	return windows.SetInformationJobObject(
		handle,
		windows.JobObjectExtendedLimitInformation,
		uintptr(unsafe.Pointer(&info)),
		uint32(unsafe.Sizeof(info)))
}
