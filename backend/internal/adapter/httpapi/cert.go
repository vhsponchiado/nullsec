package httpapi

import "net/http"

func (a *API) downloadCACert(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/x-pem-file")
	w.Header().Set("Content-Disposition", `attachment; filename="nullsec-ca-cert.pem"`)
	_, _ = w.Write(a.certificate.DownloadCA())
}

func (a *API) regenerateCACert(w http.ResponseWriter, r *http.Request) {
	if err := a.certificate.RegenerateCA(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]string{"status": "regenerated"})
}
