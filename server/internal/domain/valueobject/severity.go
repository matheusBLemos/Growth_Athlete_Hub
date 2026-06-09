package valueobject

import "errors"

var ErrInvalidSeverity = errors.New("invalid severity")

type Severity string

const (
	SeverityInfo     Severity = "info"
	SeverityWarning  Severity = "warning"
	SeverityCritical Severity = "critical"
)

var validSeverities = map[Severity]bool{
	SeverityInfo:     true,
	SeverityWarning:  true,
	SeverityCritical: true,
}

func NewSeverity(s string) (Severity, error) {
	sev := Severity(s)
	if !validSeverities[sev] {
		return "", ErrInvalidSeverity
	}
	return sev, nil
}
