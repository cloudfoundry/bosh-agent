// Copyright 2013 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ps

import (
	"syscall"
	"unsafe"

	"github.com/greenhouse-org/ps/winapi"
)

// JobObjectMonitor job objects events (like new process
// started or existing process ended).
type JobObjectMonitor struct {
	jo     *JobObject
	Handle syscall.Handle
}

// Monitor creates new event monitor for job object jo.
func (jo *JobObject) Monitor() (*JobObjectMonitor, error) {
	h, err := syscall.CreateIoCompletionPort(syscall.InvalidHandle, 0, 0, 1)
	if err != nil {
		return nil, err
	}
	port := winapi.JOBOBJECT_ASSOCIATE_COMPLETION_PORT{
		CompletionKey:  uintptr(jo.Handle),
		CompletionPort: h,
	}
	err = winapi.SetInformationJobObject(jo.Handle, winapi.JobObjectAssociateCompletionPortInformation,
		uintptr(unsafe.Pointer(&port)), uint32(unsafe.Sizeof(port)))
	if err != nil {
		syscall.CloseHandle(h)
		return nil, err
	}
	return &JobObjectMonitor{jo: jo, Handle: h}, nil
}

// GetEvent waits for next m's event. Return parameters
// are event specific - see source code for details.
func (m *JobObjectMonitor) GetEvent() (uint32, uintptr, error) {
	var code, key uint32
	var o *syscall.Overlapped
	err := syscall.GetQueuedCompletionStatus(m.Handle, &code, &key, &o, syscall.INFINITE)
	if err != nil {
		return 0, 0, err
	}
	if key != uint32(m.jo.Handle) {
		panic("Invalid GetQueuedCompletionStatus key parameter")
	}
	return code, uintptr(unsafe.Pointer(o)), nil
}

// Close releases monitor resources.
func (m *JobObjectMonitor) Close() error {
	return syscall.CloseHandle(m.Handle)
}
