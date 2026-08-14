package httpserver

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/marcosnikel/cadana-disbursement-tool/backend/internal/openapi"
)

const maximumRequestBodyBytes = 64 << 10

func (s *Server) writeError(
	responseWriter http.ResponseWriter,
	request *http.Request,
	statusCode int,
	code openapi.ErrorCode,
	message string,
) {
	s.writeJSON(responseWriter, statusCode, newErrorResponse(request, code, message))
}

func newErrorResponse(
	request *http.Request,
	code openapi.ErrorCode,
	message string,
) openapi.ErrorResponse {
	return openapi.ErrorResponse{
		Code:      code,
		Message:   message,
		RequestID: requestIDFrom(request),
	}
}

func (s *Server) writeJSON(responseWriter http.ResponseWriter, statusCode int, body any) {
	responseWriter.Header().Set("Content-Type", "application/json")
	responseWriter.WriteHeader(statusCode)
	if err := json.NewEncoder(responseWriter).Encode(body); err != nil {
		s.logger.Error("encode JSON response", "error", err)
	}
}

func decodeJSONBody(responseWriter http.ResponseWriter, request *http.Request, destination any) error {
	request.Body = http.MaxBytesReader(responseWriter, request.Body, maximumRequestBodyBytes)
	decoder := json.NewDecoder(request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("request body must contain one JSON object")
	}
	return nil
}
