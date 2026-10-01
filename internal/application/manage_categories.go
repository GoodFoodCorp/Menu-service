package application

import (
	"context"

	"goodfood/menu-service/internal/domain"
)

// ListCategories is public — every dish form (franchisee or admin) needs the
// list to populate its category picker.
func (uc *UseCases) ListCategories(ctx context.Context) ([]domain.Category, error) {
	return uc.categories.List(ctx)
}

// CreateCategory is head-office only (root README §5.4: categories are a
// network-wide taxonomy, not something a single franchisee owns).
func (uc *UseCases) CreateCategory(ctx context.Context, actor Actor, name string) (*domain.Category, error) {
	if !actor.HasRole(RoleAdmin) {
		return nil, domain.NewForbiddenError("only head office can manage categories")
	}
	maxOrder, err := uc.categories.MaxSortOrder(ctx)
	if err != nil {
		return nil, err
	}
	category, err := domain.NewCategory(name, maxOrder+1)
	if err != nil {
		return nil, err
	}
	if err := uc.categories.Create(ctx, category); err != nil {
		return nil, err
	}
	return category, nil
}

// DeleteCategory is head-office only. Dishes already using the category keep
// their category string — it's a free label, not a foreign key.
func (uc *UseCases) DeleteCategory(ctx context.Context, actor Actor, id string) error {
	if !actor.HasRole(RoleAdmin) {
		return domain.NewForbiddenError("only head office can manage categories")
	}
	return uc.categories.Delete(ctx, id)
}
