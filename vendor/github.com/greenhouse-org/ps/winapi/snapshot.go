// Copyright 2012 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package winapi

const (
	TH32CS_SNAPPROCESS = 0x00000002
)

type PROCESSENTRY32 struct {
	Size            uint32
	Usage           uint32
	ProcessID       uint32
	DefaultHeapID   uintptr
	ModuleID        uint32
	Threads         uint32
	ParentProcessID uint32
	PriClassBase    int32
	Flags           uint32
	ExeFile         [260]byte
}

//sys	CreateToolhelp32Snapshot(flags uint32, pid uint32) (handle syscall.Handle, err error) = kernel32.CreateToolhelp32Snapshot
//sys	Process32First(snapshot syscall.Handle, lppe *PROCESSENTRY32) (err error) = kernel32.Process32First
//sys	Process32Next(snapshot syscall.Handle, lppe *PROCESSENTRY32) (err error) = kernel32.Process32Next
