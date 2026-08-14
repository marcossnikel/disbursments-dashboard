package httpserver

import (
	"net/http"

	"github.com/marcosnikel/cadana-disbursement-tool/backend/internal/openapi"
)

func (s *Server) handleListWorkers(responseWriter http.ResponseWriter, _ *http.Request) {
	workers := s.processor.AvailableWorkers()
	response := make([]openapi.Worker, 0, len(workers))
	for _, worker := range workers {
		response = append(response, openapi.Worker{
			ID:       string(worker.ID()),
			Name:     worker.Name(),
			Amount:   worker.Amount().String(),
			Currency: openapi.Currency(worker.Amount().Currency()),
		})
	}

	s.writeJSON(responseWriter, http.StatusOK, response)
}
