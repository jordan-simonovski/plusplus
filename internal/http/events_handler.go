package http

import "net/http"

type EventProcessor interface {
	ProcessEvent(http.ResponseWriter, *http.Request)
}

type eventsHandler struct {
	processor EventProcessor
}

func NewEventsHandler(processor EventProcessor) *eventsHandler {
	return &eventsHandler{processor: processor}
}

func (h *eventsHandler) HandleEvents(w http.ResponseWriter, r *http.Request) {
	h.processor.ProcessEvent(w, r)
}
