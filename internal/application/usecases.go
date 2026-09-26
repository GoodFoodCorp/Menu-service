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

type UseCases struct {
	menu domain.MenuRepository
}

func NewUseCases(menu domain.MenuRepository) *UseCases {
	return &UseCases{menu: menu}
}

// ListRestaurantMenu returns the available items of one restaurant (public,
// customer-facing). No authorization: the storefront shows it before login.
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
