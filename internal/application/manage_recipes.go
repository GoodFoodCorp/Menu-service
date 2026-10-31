package application

import (
	"context"

	"goodfood/menu-service/internal/domain"
)

// recipeScope returns the restaurant whose recipe the actor edits. Only a
// restaurant manager may — head office (admin) gets 403, since recipes are
// per-restaurant data.
func recipeScope(actor Actor) (string, error) {
	if actor.HasRole(RoleManager) && actor.TenantID != "" {
		return domain.NormalizeUUID("restaurant id", actor.TenantID)
	}
	if actor.HasRole(RoleManager) {
		return "", domain.NewForbiddenError("your account is not linked to a restaurant")
	}
	return "", domain.NewForbiddenError("only a restaurant manager can manage recipes")
}

// recipeScopeForItem resolves the actor's restaurant and checks the dish
// exists and is either global or owned by that restaurant. It returns the
// normalised restaurant and dish ids (safe to pass to SQL uuid columns).
func (uc *UseCases) recipeScopeForItem(ctx context.Context, actor Actor, itemID string) (tenantID, normItemID string, err error) {
	if tenantID, err = recipeScope(actor); err != nil {
		return "", "", err
	}
	if normItemID, err = domain.NormalizeUUID("menu item id", itemID); err != nil {
		return "", "", err
	}
	item, err := uc.menu.GetByID(ctx, normItemID)
	if err != nil {
		return "", "", err
	}
	if !item.IsGlobal() && !item.IsOwnedBy(tenantID) {
		return "", "", domain.NewForbiddenError("this item is not yours")
	}
	return tenantID, normItemID, nil
}

// GetRecipe returns the actor's restaurant's recipe for a dish (empty if none).
func (uc *UseCases) GetRecipe(ctx context.Context, actor Actor, itemID string) ([]domain.RecipeLine, error) {
	tenantID, itemID, err := uc.recipeScopeForItem(ctx, actor, itemID)
	if err != nil {
		return nil, err
	}
	return uc.recipes.Get(ctx, tenantID, itemID)
}

// SetRecipe replaces the actor's restaurant's recipe for a dish; empty lines
// delete it. Quantities are rounded to 3 decimals before validation.
func (uc *UseCases) SetRecipe(ctx context.Context, actor Actor, itemID string, lines []domain.RecipeLine) error {
	tenantID, itemID, err := uc.recipeScopeForItem(ctx, actor, itemID)
	if err != nil {
		return err
	}
	rounded := make([]domain.RecipeLine, len(lines))
	for i, l := range lines {
		l.Quantity = domain.RoundQuantity(l.Quantity)
		rounded[i] = l
	}
	if err := domain.ValidateRecipe(rounded); err != nil {
		return err
	}
	return uc.recipes.Replace(ctx, tenantID, itemID, rounded)
}

// LookupRecipes returns a restaurant's recipes for several dishes (internal
// use by Order-service); dishes without a recipe are omitted.
func (uc *UseCases) LookupRecipes(ctx context.Context, restaurantID string, itemIDs []string) (map[string][]domain.RecipeLine, error) {
	if restaurantID == "" {
		return nil, domain.NewValidationError("restaurantId is required")
	}
	restaurantID, err := domain.NormalizeUUID("restaurantId", restaurantID)
	if err != nil {
		return nil, err
	}
	ids := make([]string, len(itemIDs))
	for i, id := range itemIDs {
		if ids[i], err = domain.NormalizeUUID("menu item id", id); err != nil {
			return nil, err
		}
	}
	if len(ids) == 0 {
		return map[string][]domain.RecipeLine{}, nil
	}
	recipes, err := uc.recipes.Lookup(ctx, restaurantID, ids)
	if err != nil {
		return nil, err
	}
	return uc.withFormuleRecipes(ctx, restaurantID, ids, recipes)
}

// withFormuleRecipes gives each ordered formule the recipe of its dishes, so
// that selling a formule consumes (and checks) the same stock as selling its
// dishes one by one.
func (uc *UseCases) withFormuleRecipes(ctx context.Context, restaurantID string, ids []string, recipes map[string][]domain.RecipeLine) (map[string][]domain.RecipeLine, error) {
	wanted := map[string]bool{}
	for _, id := range ids {
		if _, ok := recipes[id]; !ok {
			wanted[id] = true
		}
	}
	if len(wanted) == 0 || uc.plans == nil {
		return recipes, nil
	}
	plans, err := uc.plans.ListAvailableByTenant(ctx, restaurantID)
	if err != nil {
		return nil, err
	}
	var formules []domain.MenuPlan
	var dishIDs []string
	for _, p := range plans {
		if wanted[p.ID] {
			formules = append(formules, p)
			dishIDs = append(dishIDs, p.DishIDs...)
		}
	}
	if len(formules) == 0 {
		return recipes, nil
	}
	dishRecipes, err := uc.recipes.Lookup(ctx, restaurantID, dishIDs)
	if err != nil {
		return nil, err
	}
	for _, p := range formules {
		parts := make([][]domain.RecipeLine, 0, len(p.DishIDs))
		for _, dishID := range p.DishIDs {
			parts = append(parts, dishRecipes[dishID])
		}
		if lines := domain.SumRecipes(parts...); len(lines) > 0 {
			recipes[p.ID] = lines
		}
	}
	return recipes, nil
}

// StockItemUsage returns, for a restaurant, the names of the dishes whose
// recipe uses each stock item (internal use by Stock-service, to refuse
// deleting or rewriting an article a recipe depends on); unused stock items
// are omitted.
func (uc *UseCases) StockItemUsage(ctx context.Context, restaurantID string, stockItemIDs []string) (map[string][]string, error) {
	if restaurantID == "" {
		return nil, domain.NewValidationError("restaurantId is required")
	}
	restaurantID, err := domain.NormalizeUUID("restaurantId", restaurantID)
	if err != nil {
		return nil, err
	}
	ids := make([]string, len(stockItemIDs))
	for i, id := range stockItemIDs {
		if ids[i], err = domain.NormalizeUUID("stock item id", id); err != nil {
			return nil, err
		}
	}
	if len(ids) == 0 {
		return map[string][]string{}, nil
	}
	return uc.recipes.DishesUsingStockItems(ctx, restaurantID, ids)
}
