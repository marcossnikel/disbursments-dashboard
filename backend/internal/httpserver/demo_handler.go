package httpserver

import (
	"errors"
	"net/http"

	"github.com/marcosnikel/cadana-disbursement-tool/backend/internal/disbursement"
	"github.com/marcosnikel/cadana-disbursement-tool/backend/internal/openapi"
)

func (s *Server) handleResetDemo(responseWriter http.ResponseWriter, request *http.Request) {
	if err := s.processor.ResetDemo(); err != nil {
		if errors.Is(err, disbursement.ErrDemoResetInProgress) {
			s.writeError(
				responseWriter,
				request,
				http.StatusConflict,
				openapi.ErrorCodeDemoResetInProgress,
				"Wait for the active batch to finish before resetting demo data.",
			)
			return
		}
		s.logger.ErrorContext(
			request.Context(),
			"reset demo state",
			"request_id", requestIDFrom(request),
			"error", err,
		)
		s.writeError(
			responseWriter,
			request,
			http.StatusInternalServerError,
			openapi.ErrorCodeInternalError,
			"The demo data could not be reset.",
		)
		return
	}

	responseWriter.WriteHeader(http.StatusNoContent)
}
