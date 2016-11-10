// Copyright 2013 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ps_test

import (
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"

	"github.com/greenhouse-org/ps"
	"github.com/greenhouse-org/ps/winapi"
)

type children struct {
	mu    sync.Mutex
	procs []*ps.Process
}

func newChildren() *children {
	return &children{
		procs: make([]*ps.Process, 0),
	}
}

func (c *children) add(t *testing.T, pid int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	p, err := ps.OpenProcess(pid)
	if err != nil {
		t.Fatalf("ps.OpenProcess failed: %v", err)
	}
	c.procs = append(c.procs, p)
}

func (c *children) printAll(t *testing.T) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, p := range c.procs {
		s, err := p.Stats()
		if err != nil {
			t.Fatalf("p.Stats failed: %v", err)
		}
		t.Logf("handle=%d stats=%+v\n", p.Handle, s)
	}
}

func (c *children) closeAll() {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, p := range c.procs {
		p.Close()
	}
}

func TestJobObjectMonitor(t *testing.T) {
	jo, err := ps.CreateJobObject("")
	if err != nil {
		t.Fatalf("ps.CreateJobObject failed: %v", err)
	}
	defer jo.Close()

	m, err := jo.Monitor()
	if err != nil {
		t.Fatalf("jo.AddMonitor failed: %v", err)
	}
	defer m.Close()

	c := newChildren()

	go func() {
		// collect all new processes as they get started
		for {
			// TODO: this loop does not exit properly, but it is ok for now
			event, o, err := m.GetEvent()
			if err != nil {
				t.Fatalf("m.GetEvent failed: %v", err)
			}
			switch event {
			case winapi.JOB_OBJECT_MSG_NEW_PROCESS:
				c.add(t, int(o))
				t.Logf("new process(pid=%d) started\n", o)
			case winapi.JOB_OBJECT_MSG_EXIT_PROCESS:
				t.Logf("existing process(pid=%d) exited\n", o)
			default:
				t.Fatalf("m.GetEvent returns unexpected message (id=%d)", o)
			}
		}
	}()

	err = jo.AddCurrentProcess()
	if err != nil {
		t.Fatalf("jo.AddCurrentProcess failed: %v", err)
	}

	runGoCmd(t)

	defer c.closeAll()
	c.printAll(t)
}

func runGoCmd(t *testing.T) string {
	const src = `
package main

func main() {
	println("hello")
}
`
	dir, err := ioutil.TempDir("", "go-build")
	if err != nil {
		t.Fatalf("failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(dir)

	path := filepath.Join(dir, "main.go")
	err = ioutil.WriteFile(path, []byte(src), 0644)
	if err != nil {
		t.Fatalf("failed to create source file: %v", err)
	}

	got, err := exec.Command("go", "run", path).CombinedOutput()
	if err != nil {
		t.Fatalf("failed to run command: %v %v", err, string(got))
	}
	return string(got)
}
