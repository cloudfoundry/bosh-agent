// Copyright 2012 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package winapi

import (
	"syscall"
)

//go:generate go run $GOROOT/src/syscall/mksyscall_windows.go -output zwinapi.go jobobj.go psapi.go

const (
	// Job object security and access rights.
	DELETE                             = 0x00010000
	READ_CONTROL                       = 0x00020000
	SYNCHRONIZE                        = 0x00100000
	WRITE_DAC                          = 0x00040000
	WRITE_OWNER                        = 0x00080000
	JOB_OBJECT_ALL_ACCESS              = 0x1F001F
	JOB_OBJECT_ASSIGN_PROCESS          = 0x0001
	JOB_OBJECT_QUERY                   = 0x0004
	JOB_OBJECT_SET_ATTRIBUTES          = 0x0002
	JOB_OBJECT_SET_SECURITY_ATTRIBUTES = 0x0010
	JOB_OBJECT_TERMINATE               = 0x0008

	JobObjectAssociateCompletionPortInformation = 7

	JOB_OBJECT_MSG_END_OF_JOB_TIME       = 1
	JOB_OBJECT_MSG_END_OF_PROCESS_TIME   = 2
	JOB_OBJECT_MSG_ACTIVE_PROCESS_LIMIT  = 3
	JOB_OBJECT_MSG_ACTIVE_PROCESS_ZERO   = 4
	JOB_OBJECT_MSG_NEW_PROCESS           = 6
	JOB_OBJECT_MSG_EXIT_PROCESS          = 7
	JOB_OBJECT_MSG_ABNORMAL_EXIT_PROCESS = 8
	JOB_OBJECT_MSG_PROCESS_MEMORY_LIMIT  = 9
	JOB_OBJECT_MSG_JOB_MEMORY_LIMIT      = 10
)

type JOBOBJECT_ASSOCIATE_COMPLETION_PORT struct {
	CompletionKey  uintptr
	CompletionPort syscall.Handle
}

//sys	CreateJobObject(jobAttrs *syscall.SecurityAttributes, name *uint16) (handle syscall.Handle, err error) = kernel32.CreateJobObjectW
//sys	OpenJobObject(desiredAccess uint32, inheritHandles bool, name *uint16) (handle syscall.Handle, err error) = kernel32.OpenJobObjectW
//sys	AssignProcessToJobObject(job syscall.Handle, process syscall.Handle) (err error) = kernel32.AssignProcessToJobObject
//sys	SetInformationJobObject(job syscall.Handle, infoclass uint32, info uintptr, infolien uint32) (err error) = kernel32.SetInformationJobObject
//sys	TerminateJobObject(job syscall.Handle, exitcode uint32) (err error) = kernel32.TerminateJobObject
