package http

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"goodfood/menu-service/internal/application"
)

// MenuHandler holds the HTTP endpoints. Handlers only decode/validate DTOs,
// call the application layer and encode responses — no business logic here.
type MenuHandler struct {
	uc *application.UseCases
}

func NewMenuHandler(uc *application.UseCases) *MenuHandler {
	return &MenuHandler{uc: uc}
}

// GET /api/menu?restaurantId=<tenantId>  (public — customer browsing)
func (h *MenuHandler) ListPublic(w http.ResponseWriter, r *http.Request) {
	restaurantID := r.URL.Query().Get("restaurantId")
	items, err := h.uc.ListRestaurantMenu(r.Context(), restaurantID)
	if err != nil {
		writeDomainError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toResponseList(items))
}

// GET /api/menu/by-ids?ids=a,b,c  (public — resolves favorited dishes)
func (h *MenuHandler) ListByIDs(w http.ResponseWriter, r *http.Request) {
	raw := r.URL.Query().Get("ids")
	if raw == "" {
		writeJSON(w, http.StatusOK, toResponseList(nil))
		return
	}
	items, err := h.uc.ListByIDs(r.Context(), strings.Split(raw, ","))
	if err != nil {
		writeDomainError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toResponseList(items))
}

// GET /api/menu/manage  (manager — own restaurant, incl. unavailable items)
func (h *MenuHandler) ListMine(w http.ResponseWriter, r *http.Request) {
	items, err := h.uc.ListMyMenu(r.Context(), actorFrom(r))
	if err != nil {
		writeDomainError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toResponseList(items))
}

// POST /api/menu  (manager — create in own restaurant)
func (h *MenuHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req menuItemRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid JSON body")
		return
	}
	item, err := h.uc.CreateItem(r.Context(), actorFrom(r), req.toInput())
	if err != nil {
		writeDomainError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, toResponse(item))
}

// PATCH /api/menu/{id}  (manager — update own item)
func (h *MenuHandler) Update(w http.ResponseWriter, r *http.Request) {
	var req menuItemRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid JSON body")
		return
	}
	item, err := h.uc.UpdateItem(r.Context(), actorFrom(r), chi.URLParam(r, "id"), req.toInput())
	if err != nil {
		writeDomainError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toResponse(item))
}

// DELETE /api/menu/{id}  (manager — delete own item)
func (h *MenuHandler) Delete(w http.ResponseWriter, r *http.Request) {
	if err := h.uc.DeleteItem(r.Context(), actorFrom(r), chi.URLParam(r, "id")); err != nil {
		writeDomainError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
