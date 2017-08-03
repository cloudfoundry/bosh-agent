// +build windows

package evtlog

import (
	"bytes"
	"errors"
	"fmt"
	"syscall"
	"unicode/utf16"
	"unicode/utf8"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
	"golang.org/x/sys/windows/svc/eventlog"
)

// MaxMsgSize is the max size of a UTF-16 formatted Event Log message
const MaxMsgSize = 31839

// EventType is a Event Log event type
type EventType uint16

const (
	ErrorType       = EventType(windows.EVENTLOG_ERROR_TYPE)
	WarningType     = EventType(windows.EVENTLOG_WARNING_TYPE)
	InformationType = EventType(windows.EVENTLOG_INFORMATION_TYPE)
)

// EventID returns the EventID corresponding to the EventType, these values were
// arbitrarily determined.
//
//   ErrorType       = 1
//   WarningType     = 1 << 1 // 2
//   InformationType = 1 << 2 // 4
//
func (e EventType) EventID() uint32 {
	switch e {
	case ErrorType:
		return 1
	case WarningType:
		return 1 << 1
	case InformationType:
		return 1 << 2
	}
	return 999 // invalid
}

// LogWriter is a Writer for writing messages to the event log.
type LogWriter struct {
	Handle    windows.Handle
	EventType EventType
	EventID   uint32 // derived from EventType
}

// OpenWriter retrieves a handle to the specified event log.
func OpenWriter(etype EventType, source string) (*LogWriter, error) {
	if etype != ErrorType && etype != WarningType && etype != InformationType {
		return nil, fmt.Errorf("invalid event type: %d", etype)
	}
	if source == "" {
		return nil, errors.New("Specify event log source")
	}
	registered, err := EventIsRegistered(source)
	if err != nil {
		return nil, err
	}
	if !registered {
		return nil, errors.New("event source does not exist: " + source)
	}
	h, err := windows.RegisterEventSource(nil, syscall.StringToUTF16Ptr(source))
	if err != nil {
		return nil, err
	}
	l := &LogWriter{
		Handle:    h,
		EventType: etype,
		EventID:   etype.EventID(),
	}
	return l, nil
}

// report writes msg to the Event Log, messages larger than MaxMsgSize when
// encoded as UTF-16 are truncated to MaxMsgSize.
func (l *LogWriter) report(etype EventType, msg []byte) error {
	u, err := BytesToUTF16(msg)
	if err != nil {
		return err
	}
	if len(u) > MaxMsgSize {
		u = u[0:MaxMsgSize]
		u[MaxMsgSize-1] = 0
	}
	ss := []*uint16{&u[0]}
	return windows.ReportEvent(l.Handle, uint16(etype), 0, l.EventID, 0, 1, 0, &ss[0], nil)
}

// Write writes UTF-8 encoded byte slice b to the Event Log, before writing b
// is converted to UTF-16 and thus must be ASCII or UTF-8 encoded.  If the
// encoded length of b is greater than MaxMsgSize, b is truncated.
func (l *LogWriter) Write(b []byte) (int, error) {
	if len(b) != 0 {
		if err := l.report(l.EventType, b); err != nil {
			return 0, err
		}
	}
	return len(b), nil
}

// Close closes event log l.
func (l *LogWriter) Close() error {
	return windows.DeregisterEventSource(l.Handle)
}

// EventIsRegistered returns if source src is registered with the Application
// Event Log.
func EventIsRegistered(src string) (bool, error) {
	const addKeyName = `SYSTEM\CurrentControlSet\Services\EventLog\Application`
	const access = registry.READ | registry.QUERY_VALUE

	appkey, err := registry.OpenKey(registry.LOCAL_MACHINE, addKeyName, access)
	if err != nil {
		return false, err
	}
	defer appkey.Close()

	// Event key - this will exist if the event is registered

	evtkey, err := registry.OpenKey(appkey, src, access)
	if err != nil {
		if err == windows.ERROR_FILE_NOT_FOUND {
			return false, nil
		}
		return false, err // unknown error
	}
	defer evtkey.Close()

	return true, nil
}

// Install modifies PC registry to allow logging with an event source src, if
// it does not already exist. It adds all required keys and values to the event
// log registry key.
func Install(src string) error {
	const supports = eventlog.Error | eventlog.Warning | eventlog.Info
	exists, err := EventIsRegistered(src)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}
	return eventlog.InstallAsEventCreate(src, supports)
}

// Remove deletes all registry elements installed by the correspondent Install.
func Remove(src string) error {
	return eventlog.Remove(src)
}

// BytesToUTF16 converts byte slice b to UTF-16.
func BytesToUTF16(b []byte) ([]uint16, error) {
	if bytes.IndexByte(b, 0) != -1 {
		return nil, errors.New("byte slice with NUL passed to BytesToUTF16Ptr")
	}
	n := utf8.RuneCount(b) + 1 // NULL
	a := make([]rune, n)
	var size int
	for i := 0; len(b) > 0; i++ {
		a[i], size = utf8.DecodeRune(b)
		b = b[size:]
	}
	return utf16.Encode(a), nil
}
