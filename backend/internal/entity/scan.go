package entity

// ScanStatus is the lifecycle shared by every long-running, cancellable
// background job — Intruder attacks, port scans, subdomain scans, and
// spider crawls. Each of those started out with its own identically-shaped
// `type Status string` before being unified here.
type ScanStatus string

const (
	ScanRunning ScanStatus = "running"
	ScanDone    ScanStatus = "done"
	ScanStopped ScanStatus = "stopped"
)
