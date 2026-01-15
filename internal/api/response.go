package api

import (
	"encoding/json"
	"net/http"

	"github.com/rs/zerolog/log"
)

// RespondWithJSON writes a JSON response
func RespondWithJSON(w http.ResponseWriter, statusCode int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	if payload != nil {
		if err := json.NewEncoder(w).Encode(payload); err != nil {
			log.Error().Err(err).Msg("Failed to encode JSON response")
		}
	}
}

// RespondWithError writes an error response
func RespondWithError(w http.ResponseWriter, statusCode int, message string) {
	RespondWithJSON(w, statusCode, ErrorResponse{
		Error:   http.StatusText(statusCode),
		Message: message,
	})
}

// RespondWithSuccess writes a success response
func RespondWithSuccess(w http.ResponseWriter, statusCode int, message string, data interface{}) {
	RespondWithJSON(w, statusCode, SuccessResponse{
		Message: message,
		Data:    data,
	})
}
