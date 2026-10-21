package application

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"goodfood/menu-service/internal/domain"
)

// fakeRecipeRepo is an in-memory RecipeRepository keyed by tenant then dish.
type fakeRecipeRepo struct {
	data map[string]map[string][]domain.RecipeLine
}

func newFakeRecipeRepo() *fakeRecipeRepo {
	return &fakeRecipeRepo{data: map[string]map[string][]domain.RecipeLine{}}
}

func (f *fakeRecipeRepo) Get(_ context.Context, tenantID, itemID string) ([]domain.RecipeLine, error) {
	return append([]domain.RecipeLine{}, f.data[tenantID][itemID]...), nil
}

func (f *fakeRecipeRepo) Replace(_ context.Context, tenantID, itemID string, lines []domain.RecipeLine) error {
	if f.data[tenantID] == nil {
		f.data[tenantID] = map[string][]domain.RecipeLine{}
	}
	if len(lines) == 0 {
		delete(f.data[tenantID], itemID)
		return nil
	}
	f.data[tenantID][itemID] = append([]domain.RecipeLine{}, lines...)
	return nil
}

func (f *fakeRecipeRepo) Lookup(_ context.Context, tenantID string, itemIDs []string) (map[string][]domain.RecipeLine, error) {
	out := map[string][]domain.RecipeLine{}
	for _, id := range itemIDs {
		if lines, ok := f.data[tenantID][id]; ok {
			out[id] = append([]domain.RecipeLine{}, lines...)
		}
	}
	return out, nil
}

func newRecipeUC() (*UseCases, *fakeRepo, *fakeRecipeRepo) {
	menu, recipes := newFakeRepo(), newFakeRecipeRepo()
	return NewUseCases(menu, newFakePlanRepo(), newFakeCategoryRepo(), recipes), menu, recipes
}

func line(stockID string, qty float64) domain.RecipeLine {
	return domain.RecipeLine{StockItemID: stockID, StockItemName: "Item " + stockID, Unit: "kg", Quantity: qty}
}

func errCode(t *testing.T, err error) domain.ErrorCode {
	t.Helper()
	var derr *domain.Error
	require.ErrorAs(t, err, &derr)
	return derr.Code
}

func TestSetRecipe_manager_on_own_dish_replaces_lines(t *testing.T) {
	uc, _, _ := newRecipeUC()
	ctx := context.Background()
	item, err := uc.CreateItem(ctx, managerA, validInput("Burger"))
	require.NoError(t, err)

	require.NoError(t, uc.SetRecipe(ctx, managerA, item.ID, []domain.RecipeLine{line("s1", 1), line("s2", 2)}))
	require.NoError(t, uc.SetRecipe(ctx, managerA, item.ID, []domain.RecipeLine{line("s3", 0.1234)}))

	got, err := uc.GetRecipe(ctx, managerA, item.ID)
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "s3", got[0].StockItemID)
	assert.Equal(t, 0.123, got[0].Quantity, "quantity is rounded to 3 decimals")
}

func TestSetRecipe_manager_on_global_dish_is_allowed_for_own_tenant_only(t *testing.T) {
	uc, _, _ := newRecipeUC()
	ctx := context.Background()
	global, err := uc.CreateItem(ctx, headOffice, validInput("Global"))
	require.NoError(t, err)

	require.NoError(t, uc.SetRecipe(ctx, managerA, global.ID, []domain.RecipeLine{line("s1", 1)}))

	a, err := uc.GetRecipe(ctx, managerA, global.ID)
	require.NoError(t, err)
	assert.Len(t, a, 1)
	b, err := uc.GetRecipe(ctx, managerB, global.ID)
	require.NoError(t, err)
	assert.Empty(t, b, "other restaurant's recipe on the same global dish is untouched")

	require.NoError(t, uc.SetRecipe(ctx, managerB, global.ID, []domain.RecipeLine{line("s9", 5)}))
	a, _ = uc.GetRecipe(ctx, managerA, global.ID)
	require.Len(t, a, 1)
	assert.Equal(t, "s1", a[0].StockItemID)
}

func TestSetRecipe_manager_on_other_restaurant_dish_is_forbidden(t *testing.T) {
	uc, _, _ := newRecipeUC()
	ctx := context.Background()
	item, err := uc.CreateItem(ctx, managerB, validInput("B dish"))
	require.NoError(t, err)

	err = uc.SetRecipe(ctx, managerA, item.ID, []domain.RecipeLine{line("s1", 1)})
	assert.Equal(t, domain.ErrCodeForbidden, errCode(t, err))
}

func TestSetRecipe_admin_is_forbidden(t *testing.T) {
	uc, _, _ := newRecipeUC()
	ctx := context.Background()
	item, err := uc.CreateItem(ctx, headOffice, validInput("Global"))
	require.NoError(t, err)

	err = uc.SetRecipe(ctx, headOffice, item.ID, []domain.RecipeLine{line("s1", 1)})
	assert.Equal(t, domain.ErrCodeForbidden, errCode(t, err))
}

func TestSetRecipe_unknown_dish_is_not_found(t *testing.T) {
	uc, _, _ := newRecipeUC()
	err := uc.SetRecipe(context.Background(), managerA, "nope", []domain.RecipeLine{line("s1", 1)})
	assert.Equal(t, domain.ErrCodeNotFound, errCode(t, err))
}

func TestSetRecipe_rejects_zero_quantity(t *testing.T) {
	uc, _, _ := newRecipeUC()
	ctx := context.Background()
	item, _ := uc.CreateItem(ctx, managerA, validInput("Burger"))

	for _, q := range []float64{0, -1, 0.0004} {
		err := uc.SetRecipe(ctx, managerA, item.ID, []domain.RecipeLine{line("s1", q)})
		assert.Equal(t, domain.ErrCodeValidation, errCode(t, err), "quantity %v", q)
	}
}

func TestSetRecipe_rejects_duplicate_stock_item(t *testing.T) {
	uc, _, _ := newRecipeUC()
	ctx := context.Background()
	item, _ := uc.CreateItem(ctx, managerA, validInput("Burger"))

	err := uc.SetRecipe(ctx, managerA, item.ID, []domain.RecipeLine{line("s1", 1), line("s1", 2)})
	assert.Equal(t, domain.ErrCodeValidation, errCode(t, err))
}

func TestSetRecipe_rejects_empty_stock_item_id(t *testing.T) {
	uc, _, _ := newRecipeUC()
	ctx := context.Background()
	item, _ := uc.CreateItem(ctx, managerA, validInput("Burger"))

	err := uc.SetRecipe(ctx, managerA, item.ID, []domain.RecipeLine{line("", 1)})
	assert.Equal(t, domain.ErrCodeValidation, errCode(t, err))
}

func TestSetRecipe_empty_lines_removes_recipe(t *testing.T) {
	uc, _, _ := newRecipeUC()
	ctx := context.Background()
	item, _ := uc.CreateItem(ctx, managerA, validInput("Burger"))
	require.NoError(t, uc.SetRecipe(ctx, managerA, item.ID, []domain.RecipeLine{line("s1", 1)}))

	require.NoError(t, uc.SetRecipe(ctx, managerA, item.ID, nil))

	got, err := uc.GetRecipe(ctx, managerA, item.ID)
	require.NoError(t, err)
	assert.Empty(t, got)
	found, err := uc.LookupRecipes(ctx, restaurantA, []string{item.ID})
	require.NoError(t, err)
	assert.Empty(t, found)
}

func TestGetRecipe_returns_only_callers_tenant_lines(t *testing.T) {
	uc, _, _ := newRecipeUC()
	ctx := context.Background()
	global, _ := uc.CreateItem(ctx, headOffice, validInput("Global"))
	require.NoError(t, uc.SetRecipe(ctx, managerA, global.ID, []domain.RecipeLine{line("sa", 1)}))
	require.NoError(t, uc.SetRecipe(ctx, managerB, global.ID, []domain.RecipeLine{line("sb", 2)}))

	got, err := uc.GetRecipe(ctx, managerB, global.ID)
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "sb", got[0].StockItemID)
}

func TestGetRecipe_other_restaurant_dish_is_forbidden(t *testing.T) {
	uc, _, _ := newRecipeUC()
	ctx := context.Background()
	item, _ := uc.CreateItem(ctx, managerB, validInput("B dish"))
	_, err := uc.GetRecipe(ctx, managerA, item.ID)
	assert.Equal(t, domain.ErrCodeForbidden, errCode(t, err))
}

func TestLookupRecipes_omits_dishes_without_recipe(t *testing.T) {
	uc, _, _ := newRecipeUC()
	ctx := context.Background()
	with, _ := uc.CreateItem(ctx, managerA, validInput("With"))
	without, _ := uc.CreateItem(ctx, managerA, validInput("Without"))
	require.NoError(t, uc.SetRecipe(ctx, managerA, with.ID, []domain.RecipeLine{line("s1", 2)}))

	got, err := uc.LookupRecipes(ctx, restaurantA, []string{with.ID, without.ID})
	require.NoError(t, err)
	assert.Len(t, got, 1)
	assert.Contains(t, got, with.ID)
	assert.NotContains(t, got, without.ID)

	_, err = uc.LookupRecipes(ctx, "", []string{with.ID})
	assert.Equal(t, domain.ErrCodeValidation, errCode(t, err))
}
