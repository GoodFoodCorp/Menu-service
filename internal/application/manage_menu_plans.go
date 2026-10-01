package application

import (
	"context"

	"goodfood/menu-service/internal/domain"
)

// ListMyMenuPlans returns every menu ("formule") the actor manages — own
// restaurant's menus plus the read-only global catalog for a franchisee,
// same rule as ListMyMenu.
func (uc *UseCases) ListMyMenuPlans(ctx context.Context, actor Actor) ([]domain.MenuPlan, error) {
	scope, err := scopeOf(actor)
	if err != nil {
		return nil, err
	}
	own, err := uc.plans.ListByScope(ctx, scope)
	if err != nil {
		return nil, err
	}
	if scope == "" {
		return own, nil
	}
	global, err := uc.plans.ListGlobalWithOverridesFor(ctx, scope)
	if err != nil {
		return nil, err
	}
	return append(own, global...), nil
}

// validateDishSelection checks every dish id exists and is visible to the
// scope building the menu (their own dishes, or the global catalog).
func (uc *UseCases) validateDishSelection(ctx context.Context, scope string, dishIDs []string) error {
	dishes, err := uc.menu.GetByIDs(ctx, dishIDs)
	if err != nil {
		return err
	}
	if len(dishes) != len(dishIDs) {
		return domain.NewValidationError("one or more selected dishes do not exist")
	}
	for _, d := range dishes {
		if !d.IsGlobal() && d.TenantID != scope {
			return domain.NewValidationError("a menu can only use your own dishes or global ones")
		}
	}
	return nil
}

// CreateMenuPlan adds a menu bundle to the actor's scope.
func (uc *UseCases) CreateMenuPlan(ctx context.Context, actor Actor, in domain.MenuPlanInput) (*domain.MenuPlan, error) {
	scope, err := scopeOf(actor)
	if err != nil {
		return nil, err
	}
	if err := uc.validateDishSelection(ctx, scope, in.DishIDs); err != nil {
		return nil, err
	}
	maxOrder, err := uc.plans.MaxSortOrder(ctx, scope)
	if err != nil {
		return nil, err
	}
	plan, err := domain.NewMenuPlan(scope, in, maxOrder+1)
	if err != nil {
		return nil, err
	}
	if err := uc.plans.Create(ctx, plan); err != nil {
		return nil, err
	}
	return plan, nil
}

// UpdateMenuPlan edits a menu bundle's own fields — only if it belongs to
// the actor's own scope.
func (uc *UseCases) UpdateMenuPlan(ctx context.Context, actor Actor, id string, in domain.MenuPlanInput) (*domain.MenuPlan, error) {
	scope, err := scopeOf(actor)
	if err != nil {
		return nil, err
	}
	plan, err := uc.plans.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if !plan.IsOwnedBy(scope) {
		return nil, domain.NewForbiddenError("this menu is not yours to edit")
	}
	if err := uc.validateDishSelection(ctx, scope, in.DishIDs); err != nil {
		return nil, err
	}
	if err := plan.ApplyUpdate(in); err != nil {
		return nil, err
	}
	if err := uc.plans.Update(ctx, plan); err != nil {
		return nil, err
	}
	return plan, nil
}

// DeleteMenuPlan removes a menu bundle outright — only if it belongs to the
// actor's own scope.
func (uc *UseCases) DeleteMenuPlan(ctx context.Context, actor Actor, id string) error {
	scope, err := scopeOf(actor)
	if err != nil {
		return err
	}
	plan, err := uc.plans.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if !plan.IsOwnedBy(scope) {
		return domain.NewForbiddenError("this menu is not yours to delete")
	}
	return uc.plans.Delete(ctx, id)
}

// ToggleMenuPlanAvailability — see ToggleItemAvailability; same rule for
// menus: own → flips Available, global (as seen by a franchisee) → toggles
// a per-restaurant override instead.
func (uc *UseCases) ToggleMenuPlanAvailability(ctx context.Context, actor Actor, id string) (*domain.MenuPlan, error) {
	scope, err := scopeOf(actor)
	if err != nil {
		return nil, err
	}
	plan, err := uc.plans.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if plan.IsOwnedBy(scope) {
		plan.Available = !plan.Available
		if err := uc.plans.Update(ctx, plan); err != nil {
			return nil, err
		}
		return plan, nil
	}

	if plan.IsGlobal() && scope != "" {
		hidden, err := uc.plans.ToggleTenantOverride(ctx, scope, id)
		if err != nil {
			return nil, err
		}
		plan.HiddenForViewer = hidden
		return plan, nil
	}

	return nil, domain.NewForbiddenError("this menu is not yours to hide")
}
