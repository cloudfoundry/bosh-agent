package syslog

// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"runtime"
	"sync"
	"testing"
	"time"
)

const TestHostname = "ipHere"

func runPktSyslog(c net.PacketConn, done chan<- string) {
	var buf [4096]byte
	var rcvd string
	ct := 0
	for {
		var n int
		var err error

		c.SetReadDeadline(time.Now().Add(100 * time.Millisecond)) //nolint:errcheck
		n, _, err = c.ReadFrom(buf[:])
		rcvd += string(buf[:n])
		if err != nil {
			if oe, ok := err.(*net.OpError); ok {
				if ct < 3 && oe.Temporary() {
					ct++
					continue
				}
			}
			break
		}
	}
	c.Close() //nolint:errcheck
	done <- rcvd
}

var crashy = false //nolint:gochecknoglobals

func testableNetwork(network string) bool {
	return network == "tcp" || network == "udp"
}

func runStreamSyslog(l net.Listener, done chan<- string, wg *sync.WaitGroup) {
	for {
		var c net.Conn
		var err error
		if c, err = l.Accept(); err != nil {
			return
		}
		wg.Add(1)
		go func(c net.Conn) {
			defer wg.Done()
			c.SetReadDeadline(time.Now().Add(time.Second)) //nolint:errcheck
			b := bufio.NewReader(c)
			for ct := 1; !crashy || ct&7 != 0; ct++ {
				s, err := b.ReadString('\n')
				if err != nil {
					break
				}
				done <- s
			}
			c.Close() //nolint:errcheck
		}(c)
	}
}

func startServer(n, listenAddr string, done chan<- string) (string, io.Closer, *sync.WaitGroup) {
	if listenAddr == "" {
		listenAddr = "127.0.0.1:0"
	}
	var addr string
	var closerSocket io.Closer

	wg := new(sync.WaitGroup)
	if n == "udp" { // TODO: can likely be simplified
		listener, err := net.ListenPacket(n, listenAddr)
		if err != nil {
			log.Fatalf("startServer failed: %v", err)
		}
		addr = listener.LocalAddr().String()
		closerSocket = listener
		wg.Add(1)
		go func() {
			defer wg.Done()
			runPktSyslog(listener, done)
		}()
	} else {
		listener, err := net.Listen(n, listenAddr)
		if err != nil {
			log.Fatalf("startServer failed: %v", err)
		}
		addr = listener.Addr().String()
		closerSocket = listener
		wg.Add(1)
		go func() {
			defer wg.Done()
			runStreamSyslog(listener, done, wg)
		}()
	}
	return addr, closerSocket, wg
}

type dialFunc func(tr, addr string) (*Writer, error)
type checkFunc func(t *testing.T, in, out string)

func testSimulated(t *testing.T, transport string, dialFn dialFunc, checkFn checkFunc) {
	const msg = "Test 123"

	done := make(chan string)
	addr, sock, srvWG := startServer(transport, "", done)
	defer srvWG.Wait()
	defer sock.Close() //nolint:errcheck

	s, err := dialFn(transport, addr)
	if err != nil {
		t.Fatalf("Dial() failed: %v", err)
	}
	if err := s.Info(msg); err != nil {
		t.Fatalf("log failed: %v", err)
	}

	checkFn(t, msg, <-done)
	s.Close() //nolint:errcheck
}

func TestWithSimulated(t *testing.T) {
	const priority = LOG_INFO | LOG_USER

	var transport []string
	for _, n := range []string{"udp", "tcp"} {
		if testableNetwork(n) {
			transport = append(transport, n)
		}
	}

	// Dial
	for _, tr := range transport {
		dialFn := func(tr, addr string) (*Writer, error) {
			return Dial(tr, addr, priority, "syslog_test")
		}
		testSimulated(t, tr, dialFn, check)
	}

	// DialHostname
	for _, tr := range transport {
		dialFn := func(tr, addr string) (*Writer, error) {
			return DialHostname(tr, addr, priority, "syslog_test", TestHostname)
		}
		checkFn := func(t *testing.T, in, out string) {
			checkHostname(t, in, out, TestHostname)
		}
		testSimulated(t, tr, dialFn, checkFn)
	}
}

func TestFlapTCP(t *testing.T) {
	const tcpNetwork = "tcp"
	if !testableNetwork(tcpNetwork) {
		t.Skipf("skipping on %s/%s; '%s' is not supported", runtime.GOOS, runtime.GOARCH, tcpNetwork)
	}

	done := make(chan string)

	// Start server
	addr, sock, srvWG := startServer(tcpNetwork, "", done)
	defer sock.Close() //nolint:errcheck

	s, err := Dial(tcpNetwork, addr, LOG_INFO|LOG_USER, "syslog_test")
	if err != nil {
		t.Fatalf("Dial() failed: %v", err)
	}

	// Send initial message
	msg := "Moo 2"
	err = s.Info(msg)
	if err != nil {
		t.Fatalf("log failed: %v", err)
	}
	check(t, msg, <-done)

	// Stop server
	sock.Close() //nolint:errcheck
	srvWG.Wait()

	// Send while server down
	msg = "Moo 3"
	err = s.Info(msg)
	if err != nil {
		t.Fatalf("log failed: %v", err)
	}

	// restart server
	addr2, sock2, srvWG2 := startServer(tcpNetwork, addr, done)
	defer srvWG2.Wait()
	defer sock2.Close() //nolint:errcheck
	if addr2 != addr {
		t.Fatalf("syslog server did not start on same port: %s", addr)
	}

	// and try retransmitting
	msg = "Moo 4"
	err = s.Info(msg)
	if err != nil {
		t.Fatalf("log failed: %v", err)
	}
	check(t, msg, <-done)

	s.Close() //nolint:errcheck
}

func TestDialHostname(t *testing.T) {
	tcpNetwork := "tcp"
	if !testableNetwork(tcpNetwork) {
		t.Skipf("skipping on %s/%s; '%s' is not supported", runtime.GOOS, runtime.GOARCH, tcpNetwork)
	}
	done := make(chan string)
	addr, sock, srvWG := startServer(tcpNetwork, "", done)
	defer srvWG.Wait()
	defer os.Remove(addr) //nolint:errcheck
	defer sock.Close()    //nolint:errcheck
	if testing.Short() {
		t.Skip("skipping syslog test during -short")
	}
	f, err := DialHostname(tcpNetwork, addr, (LOG_LOCAL7|LOG_DEBUG)+1, "syslog_test", TestHostname) //nolint:staticcheck,ineffassign
	if f != nil {
		t.Fatalf("Should have trapped bad priority")
	}
	f, err = DialHostname(tcpNetwork, addr, -1, "syslog_test", TestHostname) //nolint:staticcheck,ineffassign
	if f != nil {
		t.Fatalf("Should have trapped bad priority")
	}
	l, err := DialHostname(tcpNetwork, addr, LOG_USER|LOG_ERR, "syslog_test", TestHostname)
	if err != nil {
		t.Fatalf("Dial() failed: %s", err)
	}
	l.Close() //nolint:errcheck
	_, err = DialHostname("", "", LOG_USER|LOG_ERR, "syslog_test", TestHostname)
	if err == nil {
		t.Fatalf("Should have returned an error for empty network addresses: %s", err)
	}
}

func TestDial(t *testing.T) {
	tcpNetwork := "tcp"
	if !testableNetwork(tcpNetwork) {
		t.Skipf("skipping on %s/%s; '%s' is not supported", runtime.GOOS, runtime.GOARCH, tcpNetwork)
	}
	done := make(chan string)
	addr, sock, srvWG := startServer(tcpNetwork, "", done)
	defer srvWG.Wait()
	defer os.Remove(addr) //nolint:errcheck
	defer sock.Close()    //nolint:errcheck
	if testing.Short() {
		t.Skip("skipping syslog test during -short")
	}
	f, err := Dial(tcpNetwork, addr, (LOG_LOCAL7|LOG_DEBUG)+1, "syslog_test") //nolint:ineffassign,staticcheck
	if f != nil {
		t.Fatalf("Should have trapped bad priority")
	}
	f, err = Dial(tcpNetwork, addr, -1, "syslog_test") //nolint:ineffassign,staticcheck
	if f != nil {
		t.Fatalf("Should have trapped bad priority")
	}
	l, err := Dial(tcpNetwork, addr, LOG_USER|LOG_ERR, "syslog_test")
	if err != nil {
		t.Fatalf("Dial() failed: %s", err)
	}
	l.Close() //nolint:errcheck
	_, err = Dial("", "", LOG_USER|LOG_ERR, "syslog_test")
	if err == nil {
		t.Fatalf("Should have returned an error for empty network addresses: %s", err)
	}
}

func check(t *testing.T, in, out string) {
	hostname, _ := os.Hostname() //nolint:errcheck
	if hostname == "" {
		t.Fatal("Error retrieving hostname")
	}
	checkHostname(t, in, out, hostname)
}

func checkHostname(t *testing.T, in, out, hostname string) {
	var parsedHostname, timestamp string
	var pid int

	tmpl := fmt.Sprintf("<%d>%%s %%s syslog_test[%%d]: %s\n", LOG_USER+LOG_INFO, in)

	n, err := fmt.Sscanf(out, tmpl, &timestamp, &parsedHostname, &pid)
	if n != 3 || err != nil || hostname != parsedHostname {
		t.Errorf("Got %q, does not match template %q (%d %s) (%s - %s)", out, tmpl, n, err, hostname, parsedHostname)
	}
}

func TestWrite(t *testing.T) {
	tests := []struct {
		pri Priority
		pre string
		msg string
		exp string
	}{
		{LOG_USER | LOG_ERR, "syslog_test", "", "%s %s syslog_test[%d]: \n"},
		{LOG_USER | LOG_ERR, "syslog_test", "write test", "%s %s syslog_test[%d]: write test\n"},
		// Write should not add \n if there already is one
		{LOG_USER | LOG_ERR, "syslog_test", "write test 2\n", "%s %s syslog_test[%d]: write test 2\n"},
	}

	hostname, _ := os.Hostname() //nolint:errcheck
	if hostname == "" {
		t.Fatal("Error retrieving hostname")
	}

	for _, test := range tests {
		done := make(chan string)
		addr, sock, srvWG := startServer("udp", "", done)
		defer srvWG.Wait()
		defer sock.Close() //nolint:errcheck
		l, err := Dial("udp", addr, test.pri, test.pre)
		if err != nil {
			t.Fatalf("syslog.Dial() failed: %v", err)
		}
		defer l.Close() //nolint:errcheck
		_, err = io.WriteString(l, test.msg)
		if err != nil {
			t.Fatalf("WriteString() failed: %v", err)
		}
		rcvd := <-done
		test.exp = fmt.Sprintf("<%d>", test.pri) + test.exp
		var parsedHostname, timestamp string
		var pid int
		if n, err := fmt.Sscanf(rcvd, test.exp, &timestamp, &parsedHostname, &pid); n != 3 || err != nil || hostname != parsedHostname {
			t.Errorf("s.Info() = '%q', didn't match '%q' (%d %s)", rcvd, test.exp, n, err)
		}
	}
}

func TestConcurrentWrite(t *testing.T) {
	addr, sock, srvWG := startServer("udp", "", make(chan string, 1))
	defer srvWG.Wait()
	defer sock.Close() //nolint:errcheck
	w, err := Dial("udp", addr, LOG_USER|LOG_ERR, "how's it going?")
	if err != nil {
		t.Fatalf("syslog.Dial() failed: %v", err)
	}
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := w.Info("test")
			if err != nil {
				t.Errorf("Info() failed: %v", err)
				return
			}
		}()
	}
	wg.Wait()
}

func TestConcurrentReconnect(t *testing.T) {
	crashy = true
	defer func() { crashy = false }()

	const N = 10
	const M = 100
	tcpNetwork := "tcp"
	if !testableNetwork(tcpNetwork) {
		t.Skipf("skipping on %s/%s; 'tcp' is not supported", runtime.GOOS, runtime.GOARCH)
	}
	done := make(chan string, N*M)
	addr, sock, srvWG := startServer(tcpNetwork, "", done)

	// count all the messages arriving
	count := make(chan int)
	go func() {
		ct := 0
		for range done {
			ct++
			// we are looking for 500 out of 1000 events
			// here because lots of log messages are lost
			// in buffers (kernel and/or bufio)
			if ct > N*M/2 {
				break
			}
		}
		count <- ct
	}()

	var wg sync.WaitGroup
	wg.Add(N)
	for i := 0; i < N; i++ {
		go func() { //nolint:staticcheck
			defer wg.Done()
			w, err := Dial(tcpNetwork, addr, LOG_USER|LOG_ERR, "tag")
			if err != nil {
				t.Fatalf("syslog.Dial() failed: %v", err) //nolint:govet,staticcheck
			}
			defer w.Close() //nolint:errcheck
			for i := 0; i < M; i++ {
				err := w.Info("test")
				if err != nil {
					t.Errorf("Info() failed: %v", err)
					return
				}
			}
		}()
	}
	wg.Wait()
	sock.Close() //nolint:errcheck
	srvWG.Wait()
	close(done)

	select {
	case <-count:
	case <-time.After(100 * time.Millisecond):
		t.Error("timeout in concurrent reconnect")
	}
}

type noopConn struct{}

func (c *noopConn) Read(b []byte) (int, error)         { return len(b), nil }
func (c *noopConn) Write(b []byte) (int, error)        { return len(b), nil }
func (c *noopConn) Close() error                       { return nil }
func (c *noopConn) LocalAddr() net.Addr                { return nil }
func (c *noopConn) RemoteAddr() net.Addr               { return nil }
func (c *noopConn) SetDeadline(t time.Time) error      { return nil }
func (c *noopConn) SetReadDeadline(t time.Time) error  { return nil }
func (c *noopConn) SetWriteDeadline(t time.Time) error { return nil }

func BenchmarkFormat(b *testing.B) {
	const testString = "test"
	var w Writer
	for i := 0; i < b.N; i++ {
		w.format(LOG_INFO, "hostname", "tag", testString)
	}
}

func BenchmarkWrite(b *testing.B) {
	testString := []byte("test")
	w := Writer{
		conn: &netConn{
			conn:  &noopConn{},
			local: false,
		},
		priority: LOG_INFO,
	}
	for i := 0; i < b.N; i++ {
		w.Write(testString) //nolint:errcheck
	}
}
