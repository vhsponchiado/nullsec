package entity

// Broadcaster pushes a live event to every connected frontend client. It's
// consumed by proxy.Forward and every long-running scan use case
// (intruder, portscan, subdomain, spider); the only implementation is
// adapter/ws, but nothing above this line knows that.
type Broadcaster interface {
	Broadcast(eventType string, payload any)
}

// Event type strings, broadcast over the WebSocket hub. These are the
// contract between the use cases that call Broadcaster.Broadcast and the
// frontend, which switches on event.type.
const (
	EventRequestIntercepted = "request_intercepted"
	EventRequestForwarded   = "request_forwarded"
	EventRequestCompleted   = "request_completed"
	EventRequestDropped     = "request_dropped"

	EventResponseIntercepted = "response_intercepted"
	EventResponseForwarded   = "response_forwarded"
	EventResponseDropped     = "response_dropped"

	EventIntruderResult  = "intruder_result"
	EventIntruderDone    = "intruder_done"
	EventIntruderStopped = "intruder_stopped"

	EventPortScanResult  = "portscan_result"
	EventPortScanDone    = "portscan_done"
	EventPortScanStopped = "portscan_stopped"

	EventSubdomainResult  = "subdomain_result"
	EventSubdomainTotal   = "subdomain_total"
	EventSubdomainDone    = "subdomain_done"
	EventSubdomainStopped = "subdomain_stopped"

	EventDirFinderResult  = "dirfinder_result"
	EventDirFinderDone    = "dirfinder_done"
	EventDirFinderStopped = "dirfinder_stopped"

	EventSpiderPage    = "spider_page"
	EventSpiderDone    = "spider_done"
	EventSpiderStopped = "spider_stopped"

	EventXSSScanResult  = "xssscan_result"
	EventXSSScanDone    = "xssscan_done"
	EventXSSScanStopped = "xssscan_stopped"

	EventSQLiScanResult  = "sqliscan_result"
	EventSQLiScanDone    = "sqliscan_done"
	EventSQLiScanStopped = "sqliscan_stopped"
)
