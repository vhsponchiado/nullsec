package httpapi

import (
	"encoding/json"
	"net/http"

	"nullsec/backend/internal/usecase/decoder"
)

func (a *API) decodeOperations(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, decoder.Operations)
}

func (a *API) decode(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Op    string `json:"op"`
		Mode  string `json:"mode"`
		Input string `json:"input"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}

	var (
		output string
		err    error
	)
	switch body.Mode {
	case "encode":
		output, err = decoder.Encode(body.Op, body.Input)
	case "decode":
		output, err = decoder.Decode(body.Op, body.Input)
	default:
		http.Error(w, "mode must be 'encode' or 'decode'", http.StatusBadRequest)
		return
	}

	if err != nil {
		writeJSON(w, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, map[string]string{"output": output})
}

func (a *API) smartDecode(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Input string `json:"input"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	writeJSON(w, map[string]any{"steps": decoder.SmartDecode(body.Input)})
}
