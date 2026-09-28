// Package redact provides a self-redacting string type for values that must
// stay usable in code but must never appear in logs or other rendered output
// (for example signed blobstore URLs, which are bearer credentials).
package redact

import (
	"fmt"
	"io"
	"log/slog"
)

// Placeholder is what a Secret renders as in place of its value.
const Placeholder = "<redacted>"

// Secret is a string that redacts itself on the usual logging paths: every fmt
// verb (via Format), Stringer/GoString, and slog when the Secret is the logged
// value (via LogValue). The real value is available only via Reveal().
//
// Boundary — JSON: JSON (un)marshaling is intentionally left as the default
// string behaviour so secrets still round-trip for functional use; a Secret
// marshals to its real value. Consequently a Secret nested in a struct that is
// serialized with encoding/json is NOT redacted. In particular slog.JSONHandler
// marshals an enclosing struct passed as a log value with encoding/json, which
// bypasses Format and LogValue and emits the real value (slog.TextHandler and
// fmt are unaffected). Do not log JSON-serialized structures that contain a
// Secret — log the Secret directly, use a text/fmt-based logger, or convert the
// enclosing value to a logging-safe form. Do not add a redacting MarshalJSON
// here: it would break the required round-trip.
type Secret string

var (
	_ fmt.Formatter  = Secret("")
	_ fmt.Stringer   = Secret("")
	_ fmt.GoStringer = Secret("")
	_ slog.LogValuer = Secret("")
)

// Reveal returns the underlying secret value. Never pass the result to a logger.
func (s Secret) Reveal() string { return string(s) }

// String implements fmt.Stringer.
func (Secret) String() string { return Placeholder }

// GoString implements fmt.GoStringer so %#v is redacted too.
func (Secret) GoString() string { return Placeholder }

// Format implements fmt.Formatter, which fmt consults for every verb, so the
// value is redacted no matter how it is printed (%v, %s, %q, %x, ...).
func (Secret) Format(f fmt.State, _ rune) { _, _ = io.WriteString(f, Placeholder) }

// LogValue implements slog.LogValuer.
func (Secret) LogValue() slog.Value { return slog.StringValue(Placeholder) }
