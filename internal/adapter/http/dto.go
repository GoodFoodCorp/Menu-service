package http

import (
	"encoding/json"
	"errors"
	"net/http"

	"goodfood/menu-service/internal/domain"
)

type menuItemRequest struct {
	Name         string   `json:"name"`
	Description  string   `json:"description"`
	PriceCents   int64    `json:"price_cents"`
	Category     string   `json:"category"`
	Emoji        string   `json:"emoji"`
	Available    *bool    `json:"available"`
	Ingredients  []string `json:"ingredients"`
	ImageDataURL string   `json:"image_data_url"`
}

func (req menuItemRequest) toInput() domain.MenuItemInput {
	available := true
	if req.Available != nil {
		available = *req.Available
	}
	return domain.MenuItemInput{
		Name:         req.Name,
		Description:  req.Description,
		PriceCents:   req.PriceCents,
		Category:     req.Category,
		Emoji:        req.Emoji,
		Available:    available,
		Ingredients:  req.Ingredients,
		ImageDataURL: req.ImageDataURL,
	}
}

type menuItemResponse struct {
	ID              string   `json:"id"`
	RestaurantID    string   `json:"restaurant_id"`
	Name            string   `json:"name"`
	Description     string   `json:"description"`
	PriceCents      int64    `json:"price_cents"`
	Category        string   `json:"category"`
	Emoji           string   `json:"emoji"`
	Rating          float64  `json:"rating"`
	Available       bool     `json:"available"`
	Ingredients     []string `json:"ingredients"`
	IsGlobal        bool     `json:"is_global"`
	HiddenForViewer bool     `json:"hidden_for_viewer"`
	ImageDataURL    string   `json:"image_data_url"`
}

func toResponse(m *domain.MenuItem) menuItemResponse {
	return menuItemResponse{
		ID:              m.ID,
		RestaurantID:    m.TenantID,
		Name:            m.Name,
		Description:     m.Description,
		PriceCents:      m.PriceCents,
		Category:        m.Category,
		Emoji:           m.Emoji,
		Rating:          m.Rating,
		Available:       m.Available,
		Ingredients:     m.Ingredients,
		IsGlobal:        m.IsGlobal(),
		HiddenForViewer: m.HiddenForViewer,
		ImageDataURL:    m.ImageDataURL,
	}
}

func toResponseList(items []domain.MenuItem) []menuItemResponse {
	out := make([]menuItemResponse, 0, len(items))
	for i := range items {
		out = append(out, toResponse(&items[i]))
	}
	return out
}

type menuPlanRequest struct {
	Name         string   `json:"name"`
	Description  string   `json:"description"`
	PriceCents   int64    `json:"price_cents"`
	Emoji        string   `json:"emoji"`
	Available    *bool    `json:"available"`
	DishIDs      []string `json:"dish_ids"`
	ImageDataURL string   `json:"image_data_url"`
}

func (req menuPlanRequest) toInput() domain.MenuPlanInput {
	available := true
	if req.Available != nil {
		available = *req.Available
	}
	return domain.MenuPlanInput{
		Name:         req.Name,
		Description:  req.Description,
		PriceCents:   req.PriceCents,
		Emoji:        req.Emoji,
		Available:    available,
		DishIDs:      req.DishIDs,
		ImageDataURL: req.ImageDataURL,
	}
}

type menuPlanResponse struct {
	ID              string   `json:"id"`
	RestaurantID    string   `json:"restaurant_id"`
	Name            string   `json:"name"`
	Description     string   `json:"description"`
	PriceCents      int64    `json:"price_cents"`
	Emoji           string   `json:"emoji"`
	Available       bool     `json:"available"`
	DishIDs         []string `json:"dish_ids"`
	IsGlobal        bool     `json:"is_global"`
	HiddenForViewer bool     `json:"hidden_for_viewer"`
	ImageDataURL    string   `json:"image_data_url"`
}

func toPlanResponse(p *domain.MenuPlan) menuPlanResponse {
	return menuPlanResponse{
		ID:              p.ID,
		RestaurantID:    p.TenantID,
		Name:            p.Name,
		Description:     p.Description,
		PriceCents:      p.PriceCents,
		Emoji:           p.Emoji,
		Available:       p.Available,
		DishIDs:         p.DishIDs,
		IsGlobal:        p.IsGlobal(),
		HiddenForViewer: p.HiddenForViewer,
		ImageDataURL:    p.ImageDataURL,
	}
}

func toPlanResponseList(plans []domain.MenuPlan) []menuPlanResponse {
	out := make([]menuPlanResponse, 0, len(plans))
	for i := range plans {
		out = append(out, toPlanResponse(&plans[i]))
	}
	return out
}

type categoryRequest struct {
	Name string `json:"name"`
}

type categoryResponse struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func toCategoryResponse(c *domain.Category) categoryResponse {
	return categoryResponse{ID: c.ID, Name: c.Name}
}

func toCategoryResponseList(categories []domain.Category) []categoryResponse {
	out := make([]categoryResponse, 0, len(categories))
	for i := range categories {
		out = append(out, toCategoryResponse(&categories[i]))
	}
	return out
}

type errorResponse struct {
	Error     string `json:"error"`
	RequestID string `json:"request_id,omitempty"`
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
