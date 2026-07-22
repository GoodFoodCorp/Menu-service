package domain

import "context"

// MenuRepository is the persistence port implemented by the postgres adapter.
type MenuRepository interface {
	// ListAvailableByTenant returns a restaurant's available items (customer view).
	ListAvailableByTenant(ctx context.Context, tenantID string) ([]MenuItem, error)
	// ListByTenant returns all items of a restaurant, incl. unavailable (manager view).
	ListByTenant(ctx context.Context, tenantID string) ([]MenuItem, error)
	GetByID(ctx context.Context, id string) (*MenuItem, error)
	Create(ctx context.Context, item *MenuItem) error
	Update(ctx context.Context, item *MenuItem) error
	Delete(ctx context.Context, id string) error
	CountByTenant(ctx context.Context, tenantID string) (int, error)
	MaxSortOrder(ctx context.Context, tenantID string) (int, error)
}
