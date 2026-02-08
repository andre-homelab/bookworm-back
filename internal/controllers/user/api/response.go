package api

import (
	"encoding/json"
	"net/http"
)

type ErrorResponse struct {
	Error   string `json:"error" example:"dados inválidos"`
	Message string `json:"message,omitempty" example:"campo inválido"`
}

func respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func respondError(w http.ResponseWriter, status int, message string, err error) {
	errResp := ErrorResponse{Error: message}
	if err != nil {
		errResp.Message = err.Error()
	}
	respondJSON(w, status, errResp)
}
