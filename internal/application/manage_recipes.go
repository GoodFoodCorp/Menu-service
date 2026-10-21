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
	return uc.recipes.Lookup(ctx, restaurantID, ids)
}
