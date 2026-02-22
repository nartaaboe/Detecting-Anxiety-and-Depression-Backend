package httpapi

import (
	"encoding/json"
	"net/http"
)

type apiError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type envelope struct {
	Data  any       `json:"data,omitempty"`
	Error *apiError `json:"error,omitempty"`
}

func writeData(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(envelope{Data: data})
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(envelope{Error: &apiError{Code: code, Message: message}})
}
