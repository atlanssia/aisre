package api

import (
	"encoding/json"
	"net/http"

	"github.com/atlanssia/aisre/internal/contract"
)

func (h *handler) handleWebhook(w http.ResponseWriter, r *http.Request) {
	var payload contract.WebhookPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeError(w, http.StatusBadRequest, "invalid webhook payload", "INVALID_REQUEST")
		return
	}

	resp, err := h.svc.ProcessWebhook(r.Context(), payload)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error(), "INVALID_REQUEST")
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(resp)
}
