package application

import (
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
}

func NewUseCases(menu domain.MenuRepository, plans domain.MenuPlanRepository, categories domain.CategoryRepository) *UseCases {
	return &UseCases{menu: menu, plans: plans, categories: categories}
}

// ListRestaurantMenu returns the available dishes of one restaurant plus the
// global catalog, merged (public, customer-facing). No authorization: the
// storefront shows it before login.
func (uc *UseCases) ListRestaurantMenu(ctx context.Context, restaurantID string) ([]domain.MenuItem, error) {
	if restaurantID == "" {
		return nil, domain.NewValidationError("restaurantId is required")
	}
	return uc.menu.ListAvailableByTenant(ctx, restaurantID)
}

// ListByIDs resolves dish ids to their details (public — used to render a
// customer's favorited dishes, which may belong to different restaurants).
func (uc *UseCases) ListByIDs(ctx context.Context, ids []string) ([]domain.MenuItem, error) {
	if len(ids) == 0 {
		return []domain.MenuItem{}, nil
	}
	return uc.menu.ListByIDs(ctx, ids)
}
