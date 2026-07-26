// Package entity holds nullsec's core domain types — the data every use
// case operates on, with zero dependency on HTTP, SQLite, chromedp, or any
// other delivery/infrastructure detail. If a type here ever needs to import
// net/http or database/sql, it has drifted out of this layer.
package entity

// Severity is the single finding-severity scale shared by every
// finding-producing use case — the active scanners (WordPress, SQLi, XSS)
// and the passive scanner. One definition means the frontend can rely on
// the same three values meaning the same thing regardless of which tool
// produced a finding.
type Severity string

const (
	SeverityInfo     Severity = "info"
	SeverityWarning  Severity = "warning"
	SeverityCritical Severity = "critical"
)
