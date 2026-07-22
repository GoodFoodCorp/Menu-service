package http

import (
	"encoding/json"
	"errors"
	"net/http"

	"goodfood/menu-service/internal/domain"
)

type menuItemRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	PriceCents  int64  `json:"price_cents"`
	Category    string `json:"category"`
	Emoji       string `json:"emoji"`
	Available   *bool  `json:"available"`
}

func (req menuItemRequest) toInput() domain.MenuItemInput {
	available := true
	if req.Available != nil {
		available = *req.Available
	}
	return domain.MenuItemInput{
		Name:        req.Name,
		Description: req.Description,
		PriceCents:  req.PriceCents,
		Category:    req.Category,
		Emoji:       req.Emoji,
		Available:   available,
	}
}

type menuItemResponse struct {
	ID           string  `json:"id"`
	RestaurantID string  `json:"restaurant_id"`
	Name         string  `json:"name"`
	Description  string  `json:"description"`
	PriceCents   int64   `json:"price_cents"`
	Category     string  `json:"category"`
	Emoji        string  `json:"emoji"`
	Rating       float64 `json:"rating"`
	Available    bool    `json:"available"`
}

type errorResponse struct {
	Error     string `json:"error"`
	RequestID string `json:"request_id,omitempty"`
}

func toResponse(m *domain.MenuItem) menuItemResponse {
	return menuItemResponse{
		ID:           m.ID,
		RestaurantID: m.TenantID,
		Name:         m.Name,
		Description:  m.Description,
		PriceCents:   m.PriceCents,
		Category:     m.Category,
		Emoji:        m.Emoji,
		Rating:       m.Rating,
		Available:    m.Available,
	}
}

func toResponseList(items []domain.MenuItem) []menuItemResponse {
	out := make([]menuItemResponse, 0, len(items))
	for i := range items {
		out = append(out, toResponse(&items[i]))
	}
	return out
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, r *http.Request, status int, msg string) {
	reqID, _ := r.Context().Value(ctxKeyRequestID).(string)
	writeJSON(w, status, errorResponse{Error: msg, RequestID: reqID})
}

func writeDomainError(w http.ResponseWriter, r *http.Request, err error) {
	var derr *domain.Error
	if errors.As(err, &derr) {
		status := map[domain.ErrorCode]int{
			domain.ErrCodeValidation: http.StatusBadRequest,
			domain.ErrCodeNotFound:   http.StatusNotFound,
			domain.ErrCodeForbidden:  http.StatusForbidden,
		}[derr.Code]
		if status == 0 {
			status = http.StatusInternalServerError
		}
		writeError(w, r, status, derr.Message)
		return
	}
	writeError(w, r, http.StatusInternalServerError, "internal server error")
}
