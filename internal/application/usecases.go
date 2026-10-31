package application

import (
	"fmt"
	"context"

	"goodfood/menu-service/internal/domain"
)

// Actor is the authenticated caller (from JWT). TenantID is the manager's own
// restaurant — every CRUD operation is confined to it.
type Actor struct {
	UserID    string
	TenantID  string
	RoleSlugs []string
}

func (a Actor) HasRole(slug string) bool {
	for _, r := range a.RoleSlugs {
		if r == slug {
			return true
		}
	}
	return false
}

const (
	RoleAdmin   = "admin"
	RoleManager = "manager"
)

// scopeOf resolves which catalog scope the actor manages: their own
// restaurant for a franchisee, or the global (head-office) catalog — "" —
// for admin. Root README §6: siège creates content common to every
// franchisee, a franchisee only ever touches their own restaurant's.
func scopeOf(actor Actor) (string, error) {
	if actor.HasRole(RoleAdmin) {
		return "", nil
	}
	if actor.HasRole(RoleManager) {
		if actor.TenantID == "" {
			return "", domain.NewForbiddenError("your account is not linked to a restaurant")
		}
		return actor.TenantID, nil
	}
	return "", domain.NewForbiddenError("only a restaurant manager or head office can manage the catalog")
}

type UseCases struct {
	menu       domain.MenuRepository
	plans      domain.MenuPlanRepository
	categories domain.CategoryRepository
	recipes    domain.RecipeRepository
}

func NewUseCases(menu domain.MenuRepository, plans domain.MenuPlanRepository, categories domain.CategoryRepository, recipes domain.RecipeRepository) *UseCases {
	return &UseCases{menu: menu, plans: plans, categories: categories, recipes: recipes}
}

// ListRestaurantMenu returns the available dishes of one restaurant plus the
// global catalog, merged (public, customer-facing). No authorization: the
// storefront shows it before login.
func (uc *UseCases) ListRestaurantMenu(ctx context.Context, restaurantID string) ([]domain.MenuItem, error) {
	if restaurantID == "" {
		return nil, domain.NewValidationError("restaurantId is required")
	}
	restaurantID, err := domain.NormalizeUUID("restaurantId", restaurantID)
	if err != nil {
		return nil, err
	}
	return uc.menu.ListAvailableByTenant(ctx, restaurantID)
}

// ListRestaurantMenuPlans is the equivalent for menu plans ("formules").
func (uc *UseCases) ListRestaurantMenuPlans(ctx context.Context, restaurantID string) ([]domain.MenuPlan, error) {
	if restaurantID == "" {
		return nil, domain.NewValidationError("restaurantId is required")
	}
	restaurantID, err := domain.NormalizeUUID("restaurantId", restaurantID)
	if err != nil {
		return nil, err
	}
	return uc.plans.ListAvailableByTenant(ctx, restaurantID)
}

// MaxDishMatches bounds a dish search; MinSearchLength/MaxSearchLength bound
// what is searched (in characters).
const (
	MaxDishMatches  = 50
	MinSearchLength = 2
	MaxSearchLength = 80
)

// SearchDishes finds the available dishes of the whole network whose name or
// category contains q (case and accents ignored), each with the restaurants
// it is offered in — so a dish served in several restaurants can be ordered
// from any of them.
func (uc *UseCases) SearchDishes(ctx context.Context, q string) ([]domain.DishMatch, error) {
	folded := domain.FoldSearch(q)
	if n := len([]rune(folded)); n < MinSearchLength || n > MaxSearchLength {
		return nil, domain.NewValidationError(fmt.Sprintf("q must be %d to %d characters", MinSearchLength, MaxSearchLength))
	}
	return uc.menu.SearchAvailable(ctx, folded, MaxDishMatches)
}

// ListByIDs resolves dish ids to their details (public — used to render a
// customer's favorited dishes, which may belong to different restaurants).
// MaxIDsPerLookup bounds a by-ids request (favorites page).
const MaxIDsPerLookup = 100

func (uc *UseCases) ListByIDs(ctx context.Context, ids []string) ([]domain.MenuItem, error) {
	if len(ids) == 0 {
		return []domain.MenuItem{}, nil
	}
	if len(ids) > MaxIDsPerLookup {
		return nil, domain.NewValidationError(fmt.Sprintf("at most %d ids per request", MaxIDsPerLookup))
	}
	return uc.menu.ListByIDs(ctx, ids)
}
