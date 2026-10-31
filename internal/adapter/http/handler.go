package http

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"goodfood/menu-service/internal/application"
	"goodfood/menu-service/internal/domain"
)

const (
	// maxBodyBytes caps the JSON body of the recipe PUT and the lookup.
	maxBodyBytes = 1 << 20 // 1 MiB
	// maxLookupItemIDs bounds how many dishes one lookup may ask for.
	maxLookupItemIDs = 200
)

// decodeLimitedJSON decodes a size-capped JSON body into v and writes the
// error response (413 / 400) itself, returning false when it did.
func decodeLimitedJSON(w http.ResponseWriter, r *http.Request, v any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	err := dec.Decode(v)
	if err == nil {
		// Reject trailing data after the JSON value (a second read must hit EOF).
		var extra json.RawMessage
		if err = dec.Decode(&extra); err == nil {
			err = errors.New("unexpected data after JSON body")
		} else if errors.Is(err, io.EOF) {
			err = nil
		}
	}
	if err != nil {
		var tooBig *http.MaxBytesError
		if errors.As(err, &tooBig) {
			writeError(w, r, http.StatusRequestEntityTooLarge, "request body too large")
			return false
		}
		writeError(w, r, http.StatusBadRequest, "invalid JSON body")
		return false
	}
	return true
}

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

// GET /api/menu/search?q=burger  (public — a dish in every restaurant serving it)
func (h *MenuHandler) Search(w http.ResponseWriter, r *http.Request) {
	matches, err := h.uc.SearchDishes(r.Context(), r.URL.Query().Get("q"))
	if err != nil {
		writeDomainError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toMatchResponses(matches))
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

// ── Recipes (per-restaurant dish composition for stock tracking) ─────────

// GET /api/menu/{id}/recipe  (manager — own restaurant's recipe, empty if none)
func (h *MenuHandler) GetRecipe(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	lines, err := h.uc.GetRecipe(r.Context(), actorFrom(r), id)
	if err != nil {
		writeDomainError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toRecipeResponse(id, lines))
}

// PUT /api/menu/{id}/recipe  (manager — replaces the recipe; empty lines delete it)
func (h *MenuHandler) SetRecipe(w http.ResponseWriter, r *http.Request) {
	var req recipeRequest
	if !decodeLimitedJSON(w, r, &req) {
		return
	}
	// A missing "lines" must not be mistaken for "delete the recipe".
	if req.Lines == nil {
		writeError(w, r, http.StatusBadRequest, "lines is required (use [] to delete the recipe)")
		return
	}
	lines := make([]domain.RecipeLine, 0, len(*req.Lines))
	for _, l := range *req.Lines {
		lines = append(lines, domain.RecipeLine{StockItemID: l.StockItemID, StockItemName: l.StockItemName, Unit: l.Unit, Quantity: l.Quantity})
	}
	id := chi.URLParam(r, "id")
	if err := h.uc.SetRecipe(r.Context(), actorFrom(r), id, lines); err != nil {
		writeDomainError(w, r, err)
		return
	}
	saved, err := h.uc.GetRecipe(r.Context(), actorFrom(r), id)
	if err != nil {
		writeDomainError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toRecipeResponse(id, saved))
}

// POST /internal/recipes/lookup  (service key — Order-service resolves the
// recipes of a restaurant's dishes; untracked dishes are absent)
func (h *MenuHandler) LookupRecipes(w http.ResponseWriter, r *http.Request) {
	var req recipeLookupRequest
	if !decodeLimitedJSON(w, r, &req) {
		return
	}
	if req.RestaurantID == nil || req.MenuItemIDs == nil {
		writeError(w, r, http.StatusBadRequest, "restaurant_id and menu_item_ids are required")
		return
	}
	if len(*req.MenuItemIDs) > maxLookupItemIDs {
		writeError(w, r, http.StatusBadRequest, "too many menu_item_ids")
		return
	}
	recipes, err := h.uc.LookupRecipes(r.Context(), *req.RestaurantID, *req.MenuItemIDs)
	if err != nil {
		writeDomainError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toLookupResponse(recipes))
}

// POST /internal/recipes/usage  (service key — Stock-service asks which dishes
// of a restaurant use given stock items; unused stock items are absent)
func (h *MenuHandler) StockItemUsage(w http.ResponseWriter, r *http.Request) {
	var req stockUsageRequest
	if !decodeLimitedJSON(w, r, &req) {
		return
	}
	if req.RestaurantID == nil || req.StockItemIDs == nil {
		writeError(w, r, http.StatusBadRequest, "restaurant_id and stock_item_ids are required")
		return
	}
	if len(*req.StockItemIDs) > maxLookupItemIDs {
		writeError(w, r, http.StatusBadRequest, "too many stock_item_ids")
		return
	}
	usage, err := h.uc.StockItemUsage(r.Context(), *req.RestaurantID, *req.StockItemIDs)
	if err != nil {
		writeDomainError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, stockUsageResponse{Usage: usage})
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
