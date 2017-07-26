// +build windows

package evtlog

import (
	"fmt"
	"syscall"
	"testing"
	"time"
	"unicode/utf16"
)

type FatalfFn func(format string, args ...interface{})

func createEventSource(Fatalf FatalfFn, name string) {
	if err := Install(name); err != nil {
		Fatalf("Install failed (%s): %s", name, err)
	}
}

func deleteEventSource(Fatalf FatalfFn, name string) {
	if err := Remove(name); err != nil {
		Fatalf("Remove failed (%s): %s", name, err)
	}
}

func testEventIsRegistered(t *testing.T, source string, isRegistered bool) {
	ok, err := EventIsRegistered(source)
	if err != nil {
		t.Fatalf("EventIsRegistered (%s) failed: %s", source, err)
	}
	if ok != isRegistered {
		t.Fatalf("EventIsRegistered (%s) expected %v got: %v", source,
			isRegistered, ok)
	}
}

func TestLogWriter_LargeWrite(t *testing.T) {
	name := fmt.Sprintf("large-write-%d", time.Now().UnixNano())

	createEventSource(t.Fatalf, name)
	defer deleteEventSource(t.Fatalf, name)

	l, err := OpenWriter(InformationType, name)
	if err != nil {
		t.Fatalf("OpenWriter: failed (%s): %s", name, err)
	}
	defer l.Close()

	msg := make([]byte, MaxMsgSize+512)
	for i := 0; i < len(msg); i++ {
		msg[i] = 'A'
	}
	if _, err := l.Write(msg); err != nil {
		t.Errorf("Write: messages should be truncated to MaxMsgSize (%d): %s",
			MaxMsgSize, err)
	}
}

func TestLogWriter(t *testing.T) {

	name := fmt.Sprintf("mylog-%d", time.Now().UnixNano())

	// Test non-existent source

	testEventIsRegistered(t, name, false)
	l, err := OpenWriter(InformationType, name)
	if err == nil {
		l.Close()
		t.Fatalf("OpenWriter: expected error for non-existent source: %s", name)
	}

	// Create the event source we will use for testing

	createEventSource(t.Fatalf, name)
	defer deleteEventSource(t.Fatalf, name)

	// Test invalid EventType

	testEventIsRegistered(t, name, true)
	l, err = OpenWriter(ErrorType+100, name)
	if err == nil {
		l.Close()
		t.Fatalf("OpenWriter: expected error for invalid EventType: %d", -1)
	}

	// Test writing

	etypes := []struct {
		EventType EventType
		Message   []byte
	}{
		{InformationType, []byte("Information - Message")},
		{WarningType, []byte("Warning - Message")},
		{ErrorType, []byte("Error - Message")},
		{ErrorType, nil},
	}

	for _, x := range etypes {
		w, err := OpenWriter(x.EventType, name)
		if err != nil {
			t.Fatalf("Open failed: %s", err)
		}
		defer func(w *LogWriter) {
			if err := w.Close(); err != nil {
				t.Fatalf("Close failed: %s", err)
			}
		}(w)
		if _, err := w.Write(x.Message); err != nil {
			t.Fatalf("%s: failed: %s", x.Message, err)
		}
	}
}

func TestInstall(t *testing.T) {
	name := fmt.Sprintf("test-install-%d", time.Now().UnixNano())

	if err := Install(name); err != nil {
		t.Fatalf("Install failed (%s): %s", name, err)
	}
	defer deleteEventSource(t.Fatalf, name)

	// Make sure it does not error if the log already exists
	if err := Install(name); err != nil {
		t.Fatalf("Install failed with pre-existing source (%s): %s", name, err)
	}
}

var decodeTests = []struct {
	str   string
	valid bool
}{
	{"", true},
	{"\x00", false},

	// taken from utf8_test.go
	{"abcd", true},
	{"☺☻☹", true},
	{"\x80\x80\x80\x80", true},
	{"日a本b語ç日ð本Ê語þ日¥本¼語i日©", true},
	{"日a本b語ç日ð本Ê\x00語þ日¥本¼語i日©", false},
	{"日a本b語ç日ð本Ê語þ日¥本¼語i日©日a本b語ç日ð本Ê語þ日¥本¼語i日©日a本b語ç日ð本Ê語þ日¥本¼語i日©", true},
	{"日a本b語ç日ð本Ê語þ日¥本¼語i日©日a本b語ç日ð本Ê語þ日¥本¼語i日©日a本b語ç日ð本Ê語þ日¥本¼語i日©\x00", false},
	{"\xe1\x80\x80", true},
	{"\xed\x80\x80", true},
	{"\xed\x9f\xbf", true}, // last code point before surrogate half.
	{"\xee\x80\x80", true}, // first code point after surrogate half.
	{"\xef\xbf\xbe", true},
	{"\xef\xbf\xbf", true},

	// taken from utf16_test.go
	{"a\u007Aa", true},
	{"a\u6C34a", true},
	{"a\uFEFFa", true},
	{"a\U00010000a", true},
	{"a\U0001D11Ea", true},
	{"a\U0010FFFDa", true},
}

func utf16Equal(a, b []uint16) bool {
	if len(a) != len(b) {
		return false
	}
	for i := 0; i < len(a); i++ {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestBytesToUTF16(t *testing.T) {
	for _, x := range decodeTests {
		u, err := BytesToUTF16([]byte(x.str))
		if err != nil {
			if x.valid {
				t.Errorf("BytesToUTF16 (%+v): error decoding %s", x, err)
			}
			continue
		}

		// Check for NULL terminator
		if u[len(u)-1] != 0 {
			t.Errorf("BytesToUTF16 (%+v): result not NULL terminated", x)
		}
		u = u[:len(u)-1] // remove NULL terminator

		exp := utf16.Encode([]rune(x.str))
		if !utf16Equal(u, exp) {
			t.Errorf("BytesToUTF16 failed to match encoded string (%+v): %s",
				x, string(utf16.Decode(u)))
		}
	}
}

func BenchmarkBytesToUTF16(b *testing.B) {
	tests := make([][]byte, len(decodeTests))
	for i, x := range decodeTests {
		tests[i] = []byte(x.str)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		BytesToUTF16(tests[i%len(tests)])
	}
}

// For comparison
func BenchmarkUTF16PtrFromString(b *testing.B) {
	tests := make([][]byte, len(decodeTests))
	for i, x := range decodeTests {
		tests[i] = []byte(x.str)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		syscall.UTF16PtrFromString(string(tests[i%len(tests)]))
	}
}

// Just out of curiosity
func BenchmarkLogWriter(b *testing.B) {
	name := fmt.Sprintf("bench-%d", time.Now().UnixNano())

	tests := make([][]byte, len(decodeTests))
	for i, x := range decodeTests {
		tests[i] = []byte(x.str)
	}

	createEventSource(b.Fatalf, name)
	defer deleteEventSource(b.Fatalf, name)

	l, err := OpenWriter(ErrorType, name)
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		l.Write(tests[i%len(tests)])
	}
}
