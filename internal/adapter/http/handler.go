package http

import (
	"encoding/json"
	"net/http"

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

// ── Dishes ("plats") ──────────────────────────────────────────────────────

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

// GET /api/menu/manage  (manager: own restaurant · admin: global catalog)
func (h *MenuHandler) ListMine(w http.ResponseWriter, r *http.Request) {
	items, err := h.uc.ListMyMenu(r.Context(), actorFrom(r))
	if err != nil {
		writeDomainError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toResponseList(items))
}

// POST /api/menu  (create a dish in the actor's scope)
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

// PATCH /api/menu/{id}  (update an item of the actor's own scope)
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

// DELETE /api/menu/{id}  (delete an item of the actor's own scope)
func (h *MenuHandler) Delete(w http.ResponseWriter, r *http.Request) {
	if err := h.uc.DeleteItem(r.Context(), actorFrom(r), chi.URLParam(r, "id")); err != nil {
		writeDomainError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// PATCH /api/menu/{id}/toggle  (hide/show — own item, or a per-restaurant
// override of a global item for a franchisee)
func (h *MenuHandler) ToggleAvailability(w http.ResponseWriter, r *http.Request) {
	item, err := h.uc.ToggleItemAvailability(r.Context(), actorFrom(r), chi.URLParam(r, "id"))
	if err != nil {
		writeDomainError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toResponse(item))
}

// ── Menus ("formules" — bundles of dishes) ──────────────────────────────

// GET /api/menu-plans?restaurantId=<tenantId>  (public — customer browsing)
func (h *MenuHandler) ListPlansPublic(w http.ResponseWriter, r *http.Request) {
	restaurantID := r.URL.Query().Get("restaurantId")
	plans, err := h.uc.ListRestaurantMenuPlans(r.Context(), restaurantID)
	if err != nil {
		writeDomainError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toPlanResponseList(plans))
}

// GET /api/menu-plans/manage  (manager: own restaurant · admin: global catalog)
func (h *MenuHandler) ListPlansMine(w http.ResponseWriter, r *http.Request) {
	plans, err := h.uc.ListMyMenuPlans(r.Context(), actorFrom(r))
	if err != nil {
		writeDomainError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toPlanResponseList(plans))
}

// POST /api/menu-plans  (create a menu in the actor's scope)
func (h *MenuHandler) CreatePlan(w http.ResponseWriter, r *http.Request) {
	var req menuPlanRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid JSON body")
		return
	}
	plan, err := h.uc.CreateMenuPlan(r.Context(), actorFrom(r), req.toInput())
	if err != nil {
		writeDomainError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, toPlanResponse(plan))
}

// PATCH /api/menu-plans/{id}  (update a menu of the actor's own scope)
func (h *MenuHandler) UpdatePlan(w http.ResponseWriter, r *http.Request) {
	var req menuPlanRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid JSON body")
		return
	}
	plan, err := h.uc.UpdateMenuPlan(r.Context(), actorFrom(r), chi.URLParam(r, "id"), req.toInput())
	if err != nil {
		writeDomainError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toPlanResponse(plan))
}

// DELETE /api/menu-plans/{id}  (delete a menu of the actor's own scope)
func (h *MenuHandler) DeletePlan(w http.ResponseWriter, r *http.Request) {
	if err := h.uc.DeleteMenuPlan(r.Context(), actorFrom(r), chi.URLParam(r, "id")); err != nil {
		writeDomainError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// PATCH /api/menu-plans/{id}/toggle  (hide/show — see ToggleAvailability)
func (h *MenuHandler) TogglePlanAvailability(w http.ResponseWriter, r *http.Request) {
	plan, err := h.uc.ToggleMenuPlanAvailability(r.Context(), actorFrom(r), chi.URLParam(r, "id"))
	if err != nil {
		writeDomainError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toPlanResponse(plan))
}

// ── Categories (network-wide taxonomy, head office only) ─────────────────

// GET /api/menu-categories  (public — every dish form needs the list)
func (h *MenuHandler) ListCategories(w http.ResponseWriter, r *http.Request) {
	categories, err := h.uc.ListCategories(r.Context())
	if err != nil {
		writeDomainError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toCategoryResponseList(categories))
}

// POST /api/menu-categories  (head office only)
func (h *MenuHandler) CreateCategory(w http.ResponseWriter, r *http.Request) {
	var req categoryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid JSON body")
		return
	}
	category, err := h.uc.CreateCategory(r.Context(), actorFrom(r), req.Name)
	if err != nil {
		writeDomainError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, toCategoryResponse(category))
}

// DELETE /api/menu-categories/{id}  (head office only)
func (h *MenuHandler) DeleteCategory(w http.ResponseWriter, r *http.Request) {
	if err := h.uc.DeleteCategory(r.Context(), actorFrom(r), chi.URLParam(r, "id")); err != nil {
		writeDomainError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
