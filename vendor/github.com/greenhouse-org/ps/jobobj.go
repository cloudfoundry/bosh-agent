// Copyright 2013 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ps

import (
	"syscall"

	"github.com/greenhouse-org/ps/winapi"
)

// JobObject
type JobObject struct {
	Handle syscall.Handle
}

// CreateJobObject creates new job object named name.
func CreateJobObject(name string) (*JobObject, error) {
	h, err := winapi.CreateJobObject(nil, &(syscall.StringToUTF16(name))[0])
	if err != nil {
		return nil, err
	}
	return &JobObject{h}, nil
}

// OpenJobObject opens existing job object named name.
func OpenJobObject(name string, access uint32) (*JobObject, error) {
	h, err := winapi.OpenJobObject(access, false, &(syscall.StringToUTF16(name))[0])
	if err != nil {
		return nil, err
	}
	return &JobObject{h}, nil
}

// Close closes job object handle. The job is destroyed when its last
// handle has been closed and all associated processes have exited.
func (jo *JobObject) Close() error {
	return syscall.CloseHandle(jo.Handle)
}

// AddProcess adds process idntified by process handle p to job object jo.
func (jo *JobObject) AddProcess(p syscall.Handle) error {
	return winapi.AssignProcessToJobObject(jo.Handle, p)
}

// AddCurrentProcess adds current process to job object jo.
func (jo *JobObject) AddCurrentProcess() error {
	p, err := syscall.GetCurrentProcess()
	if err != nil {
		return err
	}
	return jo.AddProcess(p)
}

func (jo *JobObject) Terminate(exitcode uint32) error {
	return winapi.TerminateJobObject(jo.Handle, exitcode)
}
