package http

import "net/http"

type CommandProcessor interface {
	ProcessCommand(http.ResponseWriter, *http.Request)
}

type commandsHandler struct {
	processor CommandProcessor
}

func NewCommandsHandler(processor CommandProcessor) *commandsHandler {
	return &commandsHandler{processor: processor}
}

func (h *commandsHandler) HandleCommand(w http.ResponseWriter, r *http.Request) {
	h.processor.ProcessCommand(w, r)
}
