package httpapi

import (
	"net/http"
	"strconv"

	"nullsec/backend/internal/entity"
	"nullsec/backend/internal/usecase/passivescan"
)

func (a *API) passiveScan(w http.ResponseWriter, r *http.Request) {
	filter := entity.RequestFilter{
		Host:   r.URL.Query().Get("host"),
		Method: r.URL.Query().Get("method"),
	}
	if s := r.URL.Query().Get("status"); s != "" {
		if v, err := strconv.Atoi(s); err == nil {
			filter.Status = v
		}
	}

	items, err := a.history.List(filter)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	inputs := make([]passivescan.Input, len(items))
	for i, it := range items {
		inputs[i] = passivescan.Input{
			ID:              it.ID,
			Method:          it.Method,
			URL:             it.URL,
			Host:            it.Host,
			RequestHeaders:  it.Headers,
			RequestBody:     it.Body,
			Status:          it.Status,
			ResponseHeaders: it.RespHeaders,
			ResponseBody:    it.RespBody,
		}
	}

	writeJSON(w, passivescan.Run(inputs))
}
