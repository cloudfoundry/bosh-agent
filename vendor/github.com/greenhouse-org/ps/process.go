// Copyright 2013 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ps

import (
	"syscall"
	"unsafe"

	"github.com/greenhouse-org/ps/winapi"
)

// Process is used to query process statistics
type Process struct {
	Handle      syscall.Handle // process handle
	closeHandle bool           // if process handle needs to be close or not
}

// OpenProcess establishes access to OS process idenified by pid.
func OpenProcess(pid int) (*Process, error) {
	h, err := syscall.OpenProcess(syscall.PROCESS_QUERY_INFORMATION, false, uint32(pid))
	if err != nil {
		return nil, err
	}
	return &Process{Handle: h, closeHandle: true}, nil
}

// OpenCurrent establishes access to current OS process.
func OpenCurrent() (*Process, error) {
	h, err := syscall.GetCurrentProcess()
	if err != nil {
		return nil, err
	}
	return &Process{Handle: h, closeHandle: false}, nil
}

// Close closes process handle.
func (p *Process) Close() error {
	return syscall.CloseHandle(p.Handle)
}

// ProcessStats stores process statistics.
type ProcessStats struct {
	CPU    syscall.Rusage
	Memory winapi.PROCESS_MEMORY_COUNTERS
}

// Stats retrieves CPU and memory usage for process p.
// Stats can be used even for a completed process.
func (p *Process) Stats() (*ProcessStats, error) {
	var s ProcessStats
	err := syscall.GetProcessTimes(p.Handle, &s.CPU.CreationTime, &s.CPU.ExitTime, &s.CPU.KernelTime, &s.CPU.UserTime)
	if err != nil {
		return nil, err
	}
	err = winapi.GetProcessMemoryInfo(p.Handle, &s.Memory, uint32(unsafe.Sizeof(s.Memory)))
	if err != nil {
		return nil, err
	}
	return &s, nil
}
