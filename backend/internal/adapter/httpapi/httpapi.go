// Package httpapi is the REST surface the frontend talks to (mounted at
// /api/*). Handlers here are meant to stay thin: decode the request, call
// into the relevant use case, and write the response — validation,
// defaults, and business logic belong in the use case being called, not
// here.
package httpapi

import (
	"encoding/json"
	"net/http"
	"strconv"

	"nullsec/backend/internal/entity"
	certificateuc "nullsec/backend/internal/usecase/certificate"
	dirfinderuc "nullsec/backend/internal/usecase/dirfinder"
	intruderuc "nullsec/backend/internal/usecase/intruder"
	portscanuc "nullsec/backend/internal/usecase/portscan"
	proxyuc "nullsec/backend/internal/usecase/proxy"
	repeateruc "nullsec/backend/internal/usecase/repeater"
	settingsuc "nullsec/backend/internal/usecase/settings"
	spideruc "nullsec/backend/internal/usecase/spider"
	sqliscanuc "nullsec/backend/internal/usecase/sqliscan"
	subdomainuc "nullsec/backend/internal/usecase/subdomain"
	xssscanuc "nullsec/backend/internal/usecase/xssscan"
)

type API struct {
	history             entity.HistoryRepository
	interceptor         *proxyuc.Interceptor
	responseInterceptor *proxyuc.ResponseInterceptor
	certificate         *certificateuc.UseCase
	dirfinder           *dirfinderuc.Manager
	intruder            *intruderuc.Manager
	portscan            *portscanuc.Manager
	subdomain           *subdomainuc.Manager
	spider              *spideruc.Manager
	settings            *settingsuc.UseCase
	repeater            *repeateruc.UseCase
	xssscan             *xssscanuc.Manager
	sqliscan            *sqliscanuc.Manager
}

func NewHandler(
	history entity.HistoryRepository,
	interceptor *proxyuc.Interceptor,
	responseInterceptor *proxyuc.ResponseInterceptor,
	certificate *certificateuc.UseCase,
	dirfinderMgr *dirfinderuc.Manager,
	intruderMgr *intruderuc.Manager,
	portscanMgr *portscanuc.Manager,
	subdomainMgr *subdomainuc.Manager,
	spiderMgr *spideruc.Manager,
	settingsUC *settingsuc.UseCase,
	repeaterUC *repeateruc.UseCase,
	xssscanMgr *xssscanuc.Manager,
	sqliscanMgr *sqliscanuc.Manager,
) http.Handler {
	a := &API{
		history:             history,
		interceptor:         interceptor,
		responseInterceptor: responseInterceptor,
		certificate:         certificate,
		dirfinder:           dirfinderMgr,
		intruder:            intruderMgr,
		portscan:            portscanMgr,
		subdomain:           subdomainMgr,
		spider:              spiderMgr,
		settings:            settingsUC,
		repeater:            repeaterUC,
		xssscan:             xssscanMgr,
		sqliscan:            sqliscanMgr,
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/history", a.listHistory)
	mux.HandleFunc("GET /api/history/{id}", a.getHistoryItem)
	mux.HandleFunc("DELETE /api/history", a.clearHistory)

	mux.HandleFunc("POST /api/intercept/toggle", a.toggleIntercept)
	mux.HandleFunc("GET /api/intercept/status", a.interceptStatus)
	mux.HandleFunc("GET /api/intercept/pending", a.listPending)
	mux.HandleFunc("POST /api/intercept/{id}/forward", a.forwardIntercepted)
	mux.HandleFunc("POST /api/intercept/{id}/drop", a.dropIntercepted)

	mux.HandleFunc("POST /api/intercept/response/toggle", a.toggleResponseIntercept)
	mux.HandleFunc("GET /api/intercept/response/status", a.responseInterceptStatus)
	mux.HandleFunc("GET /api/intercept/response/pending", a.listPendingResponses)
	mux.HandleFunc("POST /api/intercept/response/{id}/forward", a.forwardInterceptedResponse)
	mux.HandleFunc("POST /api/intercept/response/{id}/drop", a.dropInterceptedResponse)

	mux.HandleFunc("POST /api/repeater/send", a.repeaterSend)

	mux.HandleFunc("GET /api/ca/cert", a.downloadCACert)
	mux.HandleFunc("POST /api/ca/regenerate", a.regenerateCACert)

	mux.HandleFunc("POST /api/tools/wordpress-check", a.wordpressCheck)
	mux.HandleFunc("GET /api/tools/sqli-payloads", a.sqliPayloads)
	mux.HandleFunc("POST /api/tools/sqli-check", a.sqliCheck)

	mux.HandleFunc("GET /api/tools/xss-payloads", a.xssPayloads)
	mux.HandleFunc("POST /api/tools/xss-check", a.xssCheck)
	mux.HandleFunc("POST /api/tools/xss-scan/start", a.xssscanStart)
	mux.HandleFunc("POST /api/tools/xss-scan/{id}/stop", a.xssscanStop)
	mux.HandleFunc("GET /api/tools/xss-scan/{id}", a.xssscanStatus)

	mux.HandleFunc("POST /api/tools/sqli-scan/start", a.sqliscanStart)
	mux.HandleFunc("POST /api/tools/sqli-scan/{id}/stop", a.sqliscanStop)
	mux.HandleFunc("GET /api/tools/sqli-scan/{id}", a.sqliscanStatus)

	mux.HandleFunc("GET /api/tools/passive-scan", a.passiveScan)

	mux.HandleFunc("GET /api/tools/decode-operations", a.decodeOperations)
	mux.HandleFunc("POST /api/tools/decode", a.decode)
	mux.HandleFunc("POST /api/tools/decode/smart", a.smartDecode)

	mux.HandleFunc("GET /api/tools/wordlist/passwords", a.wordlistPasswords)
	mux.HandleFunc("GET /api/tools/wordlist/usernames", a.wordlistUsernames)
	mux.HandleFunc("GET /api/tools/wordlist/credentials", a.wordlistCredentials)
	mux.HandleFunc("POST /api/tools/intruder/start", a.intruderStart)
	mux.HandleFunc("POST /api/tools/intruder/{id}/stop", a.intruderStop)
	mux.HandleFunc("GET /api/tools/intruder/{id}", a.intruderStatus)
	mux.HandleFunc("GET /api/tools/intruder/{id}/results/{index}", a.intruderResult)

	mux.HandleFunc("GET /api/tools/portscan/presets", a.portscanPresets)
	mux.HandleFunc("POST /api/tools/portscan/start", a.portscanStart)
	mux.HandleFunc("POST /api/tools/portscan/{id}/stop", a.portscanStop)
	mux.HandleFunc("GET /api/tools/portscan/{id}", a.portscanStatus)

	mux.HandleFunc("GET /api/tools/subdomain/wordlist-size", a.subdomainWordlistSize)
	mux.HandleFunc("POST /api/tools/subdomain/start", a.subdomainStart)
	mux.HandleFunc("POST /api/tools/subdomain/{id}/stop", a.subdomainStop)
	mux.HandleFunc("GET /api/tools/subdomain/{id}", a.subdomainStatus)

	mux.HandleFunc("GET /api/tools/dirfinder/wordlist-size", a.dirfinderWordlistSize)
	mux.HandleFunc("POST /api/tools/dirfinder/start", a.dirfinderStart)
	mux.HandleFunc("POST /api/tools/dirfinder/{id}/stop", a.dirfinderStop)
	mux.HandleFunc("GET /api/tools/dirfinder/{id}", a.dirfinderStatus)

	mux.HandleFunc("GET /api/settings/upstream-proxy", a.getUpstreamProxy)
	mux.HandleFunc("POST /api/settings/upstream-proxy", a.setUpstreamProxy)

	mux.HandleFunc("POST /api/tools/spider/start", a.spiderStart)
	mux.HandleFunc("POST /api/tools/spider/{id}/stop", a.spiderStop)
	mux.HandleFunc("GET /api/tools/spider/{id}", a.spiderStatus)

	return mux
}

const (
	defaultHistoryLimit = 200
	maxHistoryLimit     = 500
)

func (a *API) listHistory(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	filter := entity.RequestFilter{
		Host:   q.Get("host"),
		Method: q.Get("method"),
	}
	if s := q.Get("status"); s != "" {
		if v, err := strconv.Atoi(s); err == nil {
			filter.Status = v
		}
	}

	limit := defaultHistoryLimit
	if l := q.Get("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil && v > 0 {
			limit = v
		}
	}
	if limit > maxHistoryLimit {
		limit = maxHistoryLimit
	}
	page := 0
	if p := q.Get("page"); p != "" {
		if v, err := strconv.Atoi(p); err == nil && v > 0 {
			page = v
		}
	}
	filter.Limit = limit
	filter.Offset = page * limit

	items, err := a.history.ListSummary(filter)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	total, err := a.history.Count(filter)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, map[string]any{
		"items": items,
		"total": total,
		"page":  page,
		"limit": limit,
	})
}

func (a *API) getHistoryItem(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	item, err := a.history.Get(id)
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	writeJSON(w, item)
}

func (a *API) clearHistory(w http.ResponseWriter, r *http.Request) {
	if err := a.history.Clear(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]string{"status": "cleared"})
}

func (a *API) toggleIntercept(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Enabled bool `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	a.interceptor.SetEnabled(body.Enabled)
	writeJSON(w, map[string]bool{"enabled": body.Enabled})
}

func (a *API) interceptStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, map[string]bool{"enabled": a.interceptor.Enabled()})
}

func (a *API) listPending(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, a.interceptor.List())
}

func (a *API) forwardIntercepted(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	original, ok := a.interceptor.Get(id)
	if !ok {
		http.Error(w, "no pending request with that id", http.StatusNotFound)
		return
	}

	decision := proxyuc.Decision{
		Action:  "forward",
		Method:  original.Method,
		URL:     original.URL,
		Headers: original.Headers,
		Body:    original.Body,
	}

	var edited struct {
		Method  string      `json:"method"`
		URL     string      `json:"url"`
		Headers http.Header `json:"headers"`
		Body    string      `json:"body"`
	}
	if r.Body != nil {
		if err := json.NewDecoder(r.Body).Decode(&edited); err == nil {
			if edited.Method != "" {
				decision.Method = edited.Method
			}
			if edited.URL != "" {
				decision.URL = edited.URL
			}
			if len(edited.Headers) > 0 {
				decision.Headers = edited.Headers
			}
			if edited.Body != "" {
				decision.Body = edited.Body
			}
		}
	}

	if !a.interceptor.Resolve(id, decision) {
		http.Error(w, "request already resolved", http.StatusConflict)
		return
	}
	writeJSON(w, map[string]string{"status": "forwarded"})
}

func (a *API) dropIntercepted(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !a.interceptor.Resolve(id, proxyuc.Decision{Action: "drop"}) {
		http.Error(w, "request already resolved", http.StatusConflict)
		return
	}
	writeJSON(w, map[string]string{"status": "dropped"})
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
