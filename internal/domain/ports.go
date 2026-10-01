package domain

import "context"

// MenuRepository is the persistence port for dishes ("plats").
type MenuRepository interface {
	// ListAvailableByTenant returns a restaurant's available items, merged
	// with the global (head-office) catalog and excluding items that
	// restaurant has hidden for itself — the customer-facing view.
	ListAvailableByTenant(ctx context.Context, tenantID string) ([]MenuItem, error)
	// ListByScope returns every item of a scope (a restaurant, or "" for the
	// global catalog), incl. unavailable — the manager/admin view of their
	// *own* items only.
	ListByScope(ctx context.Context, scope string) ([]MenuItem, error)
	// ListGlobalWithOverridesFor returns the global catalog annotated with
	// HiddenForViewer for the given restaurant — the read-only "defaults"
	// section of a franchisee's management view.
	ListGlobalWithOverridesFor(ctx context.Context, tenantID string) ([]MenuItem, error)
	GetByID(ctx context.Context, id string) (*MenuItem, error)
	// ListByIDs resolves a set of item ids to their details (e.g. a customer's
	// favorited dishes, which may span several restaurants). Unknown ids are
	// silently skipped rather than erroring.
	ListByIDs(ctx context.Context, ids []string) ([]MenuItem, error)
	Create(ctx context.Context, item *MenuItem) error
	Update(ctx context.Context, item *MenuItem) error
	Delete(ctx context.Context, id string) error
	CountByTenant(ctx context.Context, tenantID string) (int, error)
	MaxSortOrder(ctx context.Context, scope string) (int, error)
	// ToggleTenantOverride hides/shows a global item for one restaurant only,
	// without touching the item itself. Returns the new hidden state.
	ToggleTenantOverride(ctx context.Context, tenantID, itemID string) (hidden bool, err error)
}

// MenuPlanRepository is the persistence port for menus ("formules" — bundles
// of dishes).
type MenuPlanRepository interface {
	ListAvailableByTenant(ctx context.Context, tenantID string) ([]MenuPlan, error)
	ListByScope(ctx context.Context, scope string) ([]MenuPlan, error)
	ListGlobalWithOverridesFor(ctx context.Context, tenantID string) ([]MenuPlan, error)
	GetByID(ctx context.Context, id string) (*MenuPlan, error)
	Create(ctx context.Context, plan *MenuPlan) error
	Update(ctx context.Context, plan *MenuPlan) error
	Delete(ctx context.Context, id string) error
	MaxSortOrder(ctx context.Context, scope string) (int, error)
	ToggleTenantOverride(ctx context.Context, tenantID, planID string) (hidden bool, err error)
}

// CategoryRepository is the persistence port for dish categories — a single
// network-wide list managed by head office (root README §6 conventions).
type CategoryRepository interface {
	List(ctx context.Context) ([]Category, error)
	Create(ctx context.Context, c *Category) error
	Delete(ctx context.Context, id string) error
	MaxSortOrder(ctx context.Context) (int, error)
}
