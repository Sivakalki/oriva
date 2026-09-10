package response

import (
	"encoding/json"
	"net/http"

	apxerrors "oriva/backend-go/errors"
)

// RespondJSON writes data as a JSON response with the given status.
func RespondJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		http.Error(w, `{"message":"internal error encoding response"}`, http.StatusInternalServerError)
	}
}

// RespondMessage writes a simple {"message": ...} body with the given status.
func RespondMessage(w http.ResponseWriter, status int, message string) {
	RespondJSON(w, status, map[string]string{"message": message})
}

// RespondError maps an *apxerrors.Error to an HTTP status and JSON body.
func RespondError(w http.ResponseWriter, err *apxerrors.Error) {
	switch err.Kind {
	case apxerrors.NotFound:
		RespondMessage(w, http.StatusNotFound, err.Message)
	case apxerrors.Conflict:
		RespondMessage(w, http.StatusConflict, err.Message)
	case apxerrors.Invalid:
		var ve apxerrors.ValidationErrors
		if err.WrappedErr != nil && apxerrors.As(err.WrappedErr, &ve) {
			RespondJSON(w, http.StatusBadRequest, map[string]any{
				"message":           err.Message,
				"validation_errors": ve,
			})
			return
		}
		RespondMessage(w, http.StatusBadRequest, err.Message)
	case apxerrors.ExpectationFailed:
		RespondMessage(w, http.StatusExpectationFailed, err.Message)
	case apxerrors.Unauthorized:
		RespondMessage(w, http.StatusUnauthorized, err.Message)
	case apxerrors.Forbidden:
		RespondMessage(w, http.StatusForbidden, err.Message)
	default:
		RespondMessage(w, http.StatusInternalServerError, err.Message)
	}
}
