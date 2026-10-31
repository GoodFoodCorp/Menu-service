package application

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"goodfood/menu-service/internal/domain"
)

func TestDishAllergens(t *testing.T) {
	uc := newUC()
	ctx := context.Background()

	in := validInput("Croque")
	in.Allergens = []string{" Lait", "gluten", "lait", ""}
	item, err := uc.CreateItem(ctx, managerA, in)
	require.NoError(t, err)
	assert.Equal(t, []string{"gluten", "lait"}, item.Allergens, "codes cleaned, sorted, once")

	in.Allergens = []string{"gluten", "kryptonite"}
	_, err = uc.CreateItem(ctx, managerA, in)
	assert.Equal(t, domain.ErrCodeValidation, errCode(t, err), "only the 14 regulatory allergens")

	in.Allergens = nil
	updated, err := uc.UpdateItem(ctx, managerA, item.ID, in)
	require.NoError(t, err)
	assert.Empty(t, updated.Allergens)
}

func TestDishTextAndImageLimits(t *testing.T) {
	uc := newUC()
	ctx := context.Background()

	long := validInput(strings.Repeat("é", 121))
	_, err := uc.CreateItem(ctx, managerA, long)
	assert.Equal(t, domain.ErrCodeValidation, errCode(t, err))

	svg := validInput("Logo")
	svg.ImageDataURL = "data:image/svg+xml;base64,PHN2ZyBvbmxvYWQ9YWxlcnQoMSk+"
	_, err = uc.CreateItem(ctx, managerA, svg)
	assert.Equal(t, domain.ErrCodeValidation, errCode(t, err), "an SVG may carry scripts")

	png := validInput("Photo")
	png.ImageDataURL = "data:image/png;base64,iVBORw0KGgo="
	_, err = uc.CreateItem(ctx, managerA, png)
	assert.NoError(t, err)
}

// Selling a formule consumes the stock of its dishes.
func TestLookupRecipes_formule_sums_its_dishes(t *testing.T) {
	uc, _, _ := newRecipeUC()
	ctx := context.Background()
	const bread, steak = "11111111-1111-1111-1111-111111111111", "22222222-2222-2222-2222-222222222222"
	burger, _ := uc.CreateItem(ctx, managerA, validInput("Burger"))
	sandwich, _ := uc.CreateItem(ctx, managerA, validInput("Sandwich"))
	require.NoError(t, uc.SetRecipe(ctx, managerA, burger.ID, []domain.RecipeLine{line(bread, 1), line(steak, 1)}))
	require.NoError(t, uc.SetRecipe(ctx, managerA, sandwich.ID, []domain.RecipeLine{line(bread, 0.5)}))
	formule, err := uc.CreateMenuPlan(ctx, managerA, domain.MenuPlanInput{
		Name: "Duo", PriceCents: 1500, Available: true, DishIDs: []string{burger.ID, sandwich.ID},
	})
	require.NoError(t, err)

	got, err := uc.LookupRecipes(ctx, restaurantA, []string{formule.ID, burger.ID})
	require.NoError(t, err)
	byStock := map[string]float64{}
	for _, l := range got[formule.ID] {
		byStock[l.StockItemID] = l.Quantity
	}
	assert.Equal(t, map[string]float64{bread: 1.5, steak: 1}, byStock)
	assert.Len(t, got[burger.ID], 2, "the dish keeps its own recipe")

	other, err := uc.LookupRecipes(ctx, restaurantB, []string{formule.ID})
	require.NoError(t, err)
	assert.Empty(t, other, "another restaurant's formule has no recipe here")
}
