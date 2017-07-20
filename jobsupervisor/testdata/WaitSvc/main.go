package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"time"

	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"
)

const Timeout = time.Minute * 2

func DeleteServices(m *mgr.Mgr) (err error) {
	if m == nil {
		m, err = mgr.Connect()
		if err != nil {
			return err
		}
		defer m.Disconnect()
	}
	names, err := m.ListServices()
	if err != nil {
		return err
	}
	for _, name := range names {
		s, err := m.OpenService(name)
		if err != nil {
			continue
		}
		c, err := s.Config()
		if err == nil && c.Description == "vcap" {
			s.Delete()
			st, err := s.Query()
			if err != nil {
				continue
			}
			if st.State != svc.Stopped || st.State != svc.StopPending {
				s.Control(svc.Stop)
			}
		}
		s.Close()
	}
	return nil
}

func ServiceNames(m *mgr.Mgr) ([]string, error) {
	list, err := m.ListServices()
	if err != nil {
		return nil, err
	}
	var names []string
	for _, name := range list {
		s, err := m.OpenService(name)
		if err != nil {
			continue
		}
		c, err := s.Config()
		if err == nil && c.Description == "vcap" {
			names = append(names, name)
		}
		s.Close()
	}
	return names, nil
}

func StopScript(filename string, interval time.Duration) error {
	m, err := mgr.Connect()
	if err != nil {
		return err
	}
	defer m.Disconnect()

	names, err := ServiceNames(m)
	if err != nil {
		return err
	}

	start := time.Now()
	for {
		n := 0
		for _, name := range names {
			if s, err := m.OpenService(name); err == nil {
				s.Close()
				n++
			}
			if n == SvcCount {
				return nil
			}
		}
		time.Sleep(interval)
		if time.Since(start) > Timeout {
			DeleteServices(m)
		}
	}
	return nil
}

func Wait(filename string, interval time.Duration) error {
	start := time.Now()
	for {
		fmt.Printf("Waiting on file: %s\n", filename)
		if _, err := os.Stat(filename); err != nil {
			return nil
		}
		time.Sleep(interval)
		if time.Since(start) > Timeout {
			return errors.New("TIMEOUT!")
		}
	}
	return nil
}

var (
	Mode     string
	StopFile string
	SvcCount int
)

func init() {
	// flag.StringVar(&Mode, "mode", "", "Mode")
	// flag.StringVar(&StopFile, "stop", "", "Stop")
	flag.IntVar(&SvcCount, "count", 1, "Count")
}

func main() {
	flag.Parse()
	if len(flag.Args()) != 2 {
		fmt.Fprintln(os.Stderr, "USAGE: MODE STOPFILE [FLAGS]")
		os.Exit(1)
	}
	mode := flag.Arg(0)
	name := flag.Arg(1)

	fmt.Println("SvcCount:", SvcCount)

	switch mode {
	case "wait":
		if err := Wait(name, time.Millisecond*100); err != nil {
			fmt.Fprintln(os.Stderr, "Error:", err)
			os.Exit(2)
		}
	case "stop":
		if err := os.Remove(name); err != nil {
			fmt.Fprintln(os.Stderr, "Error:", err)
			os.Exit(2)
		}
		if err := StopScript(name, time.Millisecond*500); err != nil {
			fmt.Fprintln(os.Stderr, "Error:", err)
			os.Exit(2)
		}
	default:
		fmt.Fprintln(os.Stderr, "USAGE: MODE STOPFILE")
		os.Exit(1)
	}
	fmt.Println("Okay!")
}
