package httpapi

import (
	"errors"
	"net/http"
	"strings"

	"github.com/nartaaboe/Detecting-Anxiety-and-Depression-Backend/internal/services"
)

func writeAppError(w http.ResponseWriter, err error) {
	if err == nil {
		writeError(w, http.StatusInternalServerError, "internal", "unknown error")
		return
	}

	status := http.StatusInternalServerError
	code := "internal"

	switch {
	case errors.Is(err, services.ErrBadRequest):
		status = http.StatusBadRequest
		code = "bad_request"
	case errors.Is(err, services.ErrUnauthorized):
		status = http.StatusUnauthorized
		code = "unauthorized"
	case errors.Is(err, services.ErrForbidden):
		status = http.StatusForbidden
		code = "forbidden"
	case errors.Is(err, services.ErrNotFound):
		status = http.StatusNotFound
		code = "not_found"
	case errors.Is(err, services.ErrConflict):
		status = http.StatusConflict
		code = "conflict"
	case errors.Is(err, services.ErrUnavailable):
		status = http.StatusServiceUnavailable
		code = "unavailable"
	}

	writeError(w, status, code, cleanMessage(err))
}

func cleanMessage(err error) string {
	msg := strings.TrimSpace(err.Error())
	for _, prefix := range []string{
		services.ErrBadRequest.Error() + ":",
		services.ErrUnauthorized.Error() + ":",
		services.ErrForbidden.Error() + ":",
		services.ErrNotFound.Error() + ":",
		services.ErrConflict.Error() + ":",
		services.ErrUnavailable.Error() + ":",
	} {
		if strings.HasPrefix(msg, prefix) {
			return strings.TrimSpace(strings.TrimPrefix(msg, prefix))
		}
	}
	return msg
}
