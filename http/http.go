package http

import (
	"context"
	"encoding/json"
	"net/http"
	"template"
)

// SessionCookieName is the name of the cookie used to store the session.
const SessionCookieName = "session"

// Session represents session data stored in a secure cookie.
type Session struct {
	UserID      int    `json:"userID"`
	RedirectURL string `json:"redirectURL"`
	State       string `json:"state"`
}

// ErrorResponse represents a JSON structure for error output.
type ErrorResponse struct {
	Error string `json:"error"`
}

// encodeError prints & optionally logs an error message.
func encodeError(w http.ResponseWriter, r *http.Request, err error) {
	// Extract error code & message.
	code, message := template.ErrorCode(err), template.ErrorMessage(err)

	// Track metrics by code.
	// errorCount.WithLabelValues(code).Inc()

	// Log & report internal errors.
	//if code == template.EINTERNAL {
	//	template.ReportError(r.Context(), err, r)
	//	LogError(r, err)
	//}

	// Print user message to response.
	w.Header().Set("Content-type", "application/json")
	w.WriteHeader(ErrorStatusCode(code))
	_ = json.NewEncoder(w).Encode(&ErrorResponse{Error: message})
}

// errorer is implemented by all concrete response types that may contain
// errors. It allows us to change the HTTP response code without needing to
// trigger an endpoint (transport-level) error.
type errorer interface {
	error() error
}

// encodeResponse is the common method to encode all response types to the
// client. I chose to do it this way because, since we're using JSON, there's no
// reason to provide anything more specific. It's certainly possible to
// specialize on a per-response (per-method) basis.
func encodeResponse(ctx context.Context, w http.ResponseWriter, response interface{}) error {
	if e, ok := response.(errorer); ok && e.error() != nil {
		// Not a Go kit transport error, but a business-logic error.
		// Provide those as HTTP errors.
		encodeError(w, nil, e.error())
		return nil
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	return json.NewEncoder(w).Encode(response)
}

// lookup of application error codes to HTTP status codes.
var codes = map[string]int{
	template.ECONFLICT:       http.StatusConflict,
	template.EINVALID:        http.StatusBadRequest,
	template.ENOTFOUND:       http.StatusNotFound,
	template.ENOTIMPLEMENTED: http.StatusNotImplemented,
	template.EUNAUTHORIZED:   http.StatusUnauthorized,
	template.EINTERNAL:       http.StatusInternalServerError,
}

// ErrorStatusCode returns the associated HTTP status code for an error code.
func ErrorStatusCode(code string) int {
	if v, ok := codes[code]; ok {
		return v
	}
	return http.StatusInternalServerError
}
