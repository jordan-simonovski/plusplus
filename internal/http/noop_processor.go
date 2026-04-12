package http

import (
	"encoding/json"
	"net/http"
)

type noopEventProcessor struct{}

func NewNoopEventProcessor() *noopEventProcessor {
	return &noopEventProcessor{}
}

func (p *noopEventProcessor) ProcessEvent(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotImplemented)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"error": "event processor is not configured",
	})
}

type noopCommandProcessor struct{}

func NewNoopCommandProcessor() *noopCommandProcessor {
	return &noopCommandProcessor{}
}

func (p *noopCommandProcessor) ProcessCommand(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotImplemented)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"error": "command processor is not configured",
	})
}
