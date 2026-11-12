// Package handler exposes the MedVault REST API.
//
// Every response is a JSON envelope carrying an "ok" flag and, when the request
// performed cryptographic work, a "_telemetry" trace for the glass box
// inspector. Handlers translate domain errors into stable HTTP status codes but
// never leak secret material into a message.
package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"

	"medvault/internal/telemetry"
)

// Envelope is the single response shape returned by the API.
type Envelope struct {
	OK        bool             `json:"ok"`
	Message   string           `json:"message,omitempty"`
	Data      any              `json:"data,omitempty"`
	Error     string           `json:"error,omitempty"`
	Telemetry *telemetry.Trace `json:"_telemetry,omitempty"`
}

// writeEnvelope serialises any envelope with the shared headers.
func writeEnvelope(w http.ResponseWriter, status int, env Envelope) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(env); err != nil {
		log.Printf("handler: write envelope: %v", err)
	}
}

// respondOK writes a success envelope.
func respondOK(w http.ResponseWriter, message string, data any, traces ...*telemetry.Trace) {
	var trace *telemetry.Trace
	switch len(traces) {
	case 0:
		trace = nil
	case 1:
		trace = traces[0]
		if trace != nil {
			trace.Finish()
		}
	default:
		trace = telemetry.Merge("API_PIPELINE", traces...)
	}

	writeEnvelope(w, http.StatusOK, Envelope{
		OK:        true,
		Message:   message,
		Data:      data,
		Telemetry: trace,
	})
}

// respondError writes a failure envelope. The message is safe for display.
func respondError(w http.ResponseWriter, status int, err error, traces ...*telemetry.Trace) {
	var trace *telemetry.Trace
	if len(traces) > 0 {
		trace = telemetry.Merge("ERROR", traces...)
	}

	writeEnvelope(w, status, Envelope{
		OK:        false,
		Error:     err.Error(),
		Telemetry: trace,
	})
}

// validationError marks a client mistake so the router can answer 400 instead
// of 500.
type validationError struct {
	msg string
}

func (e *validationError) Error() string { return e.msg }

// badRequest builds a validation error.
func badRequest(format string, args ...any) error {
	return &validationError{msg: fmt.Sprintf(format, args...)}
}

// statusFor maps a domain error to an HTTP status.
func statusFor(err error) int {
	var ve *validationError
	switch {
	case errors.As(err, &ve):
		return http.StatusBadRequest
	case errors.Is(err, errUnauthorized):
		return http.StatusUnauthorized
	case errors.Is(err, errNotFound):
		return http.StatusNotFound
	case errors.Is(err, errConflict):
		return http.StatusConflict
	case errors.Is(err, errUnavailable):
		return http.StatusServiceUnavailable
	default:
		return http.StatusInternalServerError
	}
}
