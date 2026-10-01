package application

import (
	"context"

	"goodfood/menu-service/internal/domain"
)

// ListMyMenu returns every dish the actor manages: for a franchisee, their
// own restaurant's dishes (full control) plus the read-only global catalog
// (they may only hide/show it — see ToggleItemAvailability). For admin,
// their own list *is* the global catalog, so nothing to merge.
func (uc *UseCases) ListMyMenu(ctx context.Context, actor Actor) ([]domain.MenuItem, error) {
	scope, err := scopeOf(actor)
	if err != nil {
		return nil, err
	}
	own, err := uc.menu.ListByScope(ctx, scope)
	if err != nil {
		return nil, err
	}
	if scope == "" {
		return own, nil
	}
	global, err := uc.menu.ListGlobalWithOverridesFor(ctx, scope)
	if err != nil {
		return nil, err
	}
	return append(own, global...), nil
}

// CreateItem adds a dish to the actor's scope.
func (uc *UseCases) CreateItem(ctx context.Context, actor Actor, in domain.MenuItemInput) (*domain.MenuItem, error) {
	scope, err := scopeOf(actor)
	if err != nil {
		return nil, err
	}
	maxOrder, err := uc.menu.MaxSortOrder(ctx, scope)
	if err != nil {
		return nil, err
	}
	item, err := domain.NewMenuItem(scope, in, maxOrder+1)
	if err != nil {
		return nil, err
	}
	if err := uc.menu.Create(ctx, item); err != nil {
		return nil, err
	}
	return item, nil
}

// UpdateItem edits a dish's own fields — only if it belongs to the actor's
// own scope. A franchisee can never edit a global (head-office) dish, even
// partially: see ToggleItemAvailability for what they can do to it instead.
func (uc *UseCases) UpdateItem(ctx context.Context, actor Actor, id string, in domain.MenuItemInput) (*domain.MenuItem, error) {
	scope, err := scopeOf(actor)
	if err != nil {
		return nil, err
	}
	item, err := uc.menu.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if !item.IsOwnedBy(scope) {
		return nil, domain.NewForbiddenError("this item is not yours to edit")
	}
	if err := item.ApplyUpdate(in); err != nil {
		return nil, err
	}
	if err := uc.menu.Update(ctx, item); err != nil {
		return nil, err
	}
	return item, nil
}

// DeleteItem removes a dish outright — only if it belongs to the actor's own
// scope. A franchisee can never delete a global dish.
func (uc *UseCases) DeleteItem(ctx context.Context, actor Actor, id string) error {
	scope, err := scopeOf(actor)
	if err != nil {
		return err
	}
	item, err := uc.menu.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if !item.IsOwnedBy(scope) {
		return domain.NewForbiddenError("this item is not yours to delete")
	}
	return uc.menu.Delete(ctx, id)
}

// ToggleItemAvailability hides/shows a dish:
//   - own item → flips its real Available flag (affects every restaurant
//     if global, i.e. an admin toggling their own global dish);
//   - a franchisee facing a global dish → toggles a per-restaurant override
//     instead, hiding it from their own customers only, without touching
//     the item everyone else sees.
func (uc *UseCases) ToggleItemAvailability(ctx context.Context, actor Actor, id string) (*domain.MenuItem, error) {
	scope, err := scopeOf(actor)
	if err != nil {
		return nil, err
	}
	item, err := uc.menu.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if item.IsOwnedBy(scope) {
		item.Available = !item.Available
		if err := uc.menu.Update(ctx, item); err != nil {
			return nil, err
		}
		return item, nil
	}

	if item.IsGlobal() && scope != "" {
		hidden, err := uc.menu.ToggleTenantOverride(ctx, scope, id)
		if err != nil {
			return nil, err
		}
		item.HiddenForViewer = hidden
		return item, nil
	}

	return nil, domain.NewForbiddenError("this item is not yours to hide")
}
