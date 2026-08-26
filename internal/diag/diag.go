package diag

import (
	"fmt"
	"strings"
)

// Severity of a diagnostic.
type Severity int

const (
	Error Severity = iota
	Warning
)

// Diagnostic is a single error or warning message.
type Diagnostic struct {
	Path     string
	Line     int
	Col      int
	Message  string
	Severity Severity
}

// Reporter collects diagnostics and can format them.
type Reporter struct {
	Diags []Diagnostic
}

// Errorf records an error diagnostic.
func (r *Reporter) Errorf(path string, line, col int, format string, args ...interface{}) {
	r.Diags = append(r.Diags, Diagnostic{
		Path:     path,
		Line:     line,
		Col:      col,
		Message:  fmt.Sprintf(format, args...),
		Severity: Error,
	})
}

// HasErrors returns true if there is at least one error.
func (r *Reporter) HasErrors() bool {
	for _, d := range r.Diags {
		if d.Severity == Error {
			return true
		}
	}
	return false
}

// String formats all diagnostics as human-readable output.
func (r *Reporter) String() string {
	var b strings.Builder
	for _, d := range r.Diags {
		sev := "error"
		if d.Severity == Warning {
			sev = "warning"
		}
		fmt.Fprintf(&b, "%s:%d:%d: %s: %s\n", d.Path, d.Line, d.Col, sev, d.Message)
	}
	return b.String()
}
