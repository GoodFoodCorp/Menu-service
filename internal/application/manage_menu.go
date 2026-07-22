package application

import (
	"context"

	"goodfood/menu-service/internal/domain"
)

// tenantOf resolves the restaurant a manager acts on: their own tenant.
// Managers without a tenant, or non-managers, are refused.
func tenantOf(actor Actor) (string, error) {
	if !actor.HasRole(RoleManager) {
		return "", domain.NewForbiddenError("only a restaurant manager can manage a menu")
	}
	if actor.TenantID == "" {
		return "", domain.NewForbiddenError("your account is not linked to a restaurant")
	}
	return actor.TenantID, nil
}

// ListMyMenu returns every item of the manager's restaurant (incl. unavailable).
func (uc *UseCases) ListMyMenu(ctx context.Context, actor Actor) ([]domain.MenuItem, error) {
	tenantID, err := tenantOf(actor)
	if err != nil {
		return nil, err
	}
	return uc.menu.ListByTenant(ctx, tenantID)
}

// CreateItem adds an item to the manager's own restaurant.
func (uc *UseCases) CreateItem(ctx context.Context, actor Actor, in domain.MenuItemInput) (*domain.MenuItem, error) {
	tenantID, err := tenantOf(actor)
	if err != nil {
		return nil, err
	}
	maxOrder, err := uc.menu.MaxSortOrder(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	item, err := domain.NewMenuItem(tenantID, in, maxOrder+1)
	if err != nil {
		return nil, err
	}
	if err := uc.menu.Create(ctx, item); err != nil {
		return nil, err
	}
	return item, nil
}

// UpdateItem edits an item, only if it belongs to the manager's restaurant.
func (uc *UseCases) UpdateItem(ctx context.Context, actor Actor, id string, in domain.MenuItemInput) (*domain.MenuItem, error) {
	tenantID, err := tenantOf(actor)
	if err != nil {
		return nil, err
	}
	item, err := uc.menu.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if !item.IsOwnedBy(tenantID) {
		return nil, domain.NewForbiddenError("this item belongs to another restaurant")
	}
	if err := item.ApplyUpdate(in); err != nil {
		return nil, err
	}
	if err := uc.menu.Update(ctx, item); err != nil {
		return nil, err
	}
	return item, nil
}

// DeleteItem removes an item, only if it belongs to the manager's restaurant.
func (uc *UseCases) DeleteItem(ctx context.Context, actor Actor, id string) error {
	tenantID, err := tenantOf(actor)
	if err != nil {
		return err
	}
	item, err := uc.menu.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if !item.IsOwnedBy(tenantID) {
		return domain.NewForbiddenError("this item belongs to another restaurant")
	}
	return uc.menu.Delete(ctx, id)
}
