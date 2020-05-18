package jobcontrol

import (
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
		return err
	}

	return windows.AssignProcessToJobObject(
		windows.Handle(c.jobObjectHandle),
		windows.Handle((*process)(unsafe.Pointer(c.cmd.Process)).Handle))
}

func (c *JobCmd) kill() {
	windows.GenerateConsoleCtrlEvent(windows.CTRL_BREAK_EVENT, uint32(c.cmd.Process.Pid))
}

func (c *JobCmd) terminate() {
	windows.CloseHandle(windows.Handle(c.jobObjectHandle))
}

func (c *JobCmd) createJobObject() error {
	handle, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return err
	}

	info := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{
		BasicLimitInformation: windows.JOBOBJECT_BASIC_LIMIT_INFORMATION{
			LimitFlags: windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE,
		},
	}
	_, err = windows.SetInformationJobObject(
		handle,
		windows.JobObjectExtendedLimitInformation,
		uintptr(unsafe.Pointer(&info)),
		uint32(unsafe.Sizeof(info)))

	c.jobObjectHandle = uintptr(handle)
	return err
}
