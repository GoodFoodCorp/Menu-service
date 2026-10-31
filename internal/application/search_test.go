package application

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"goodfood/menu-service/internal/domain"
)

func TestSearchFindsADishInEveryRestaurantServingIt(t *testing.T) {
	uc := newUC()
	ctx := context.Background()

	global, err := uc.CreateItem(ctx, headOffice, domain.MenuItemInput{Name: "Crème brûlée", Category: "Desserts", PriceCents: 650, Available: true})
	require.NoError(t, err)
	_, err = uc.CreateItem(ctx, managerA, domain.MenuItemInput{Name: "Brûlée maison", Category: "Desserts", PriceCents: 700, Available: true})
	require.NoError(t, err)
	_, err = uc.CreateItem(ctx, managerB, domain.MenuItemInput{Name: "Burger", Category: "Burgers", PriceCents: 1200, Available: false})
	require.NoError(t, err)
	_, err = uc.ToggleItemAvailability(ctx, managerB, global.ID) // restaurant B hides the global dish
	require.NoError(t, err)

	matches, err := uc.SearchDishes(ctx, "  BRULEE ")
	require.NoError(t, err)
	require.Len(t, matches, 2)
	for _, m := range matches {
		if m.Item.ID == global.ID {
			assert.True(t, m.Item.IsGlobal())
			assert.Equal(t, []string{restaurantB}, m.HiddenIn)
		} else {
			assert.Equal(t, restaurantA, m.Item.TenantID)
			assert.Empty(t, m.HiddenIn)
		}
	}

	// By category, and never an unavailable dish.
	desserts, err := uc.SearchDishes(ctx, "dessert")
	require.NoError(t, err)
	assert.Len(t, desserts, 2)
	burgers, err := uc.SearchDishes(ctx, "burger")
	require.NoError(t, err)
	assert.Empty(t, burgers)
}

func TestSearchNeedsTwoToEightyCharacters(t *testing.T) {
	uc := newUC()
	for _, q := range []string{"", " a ", string(make([]byte, 81))} {
		_, err := uc.SearchDishes(context.Background(), q)
		var derr *domain.Error
		require.ErrorAs(t, err, &derr, q)
		assert.Equal(t, domain.ErrCodeValidation, derr.Code)
	}
}

func TestFoldSearch(t *testing.T) {
	assert.Equal(t, "creme brulee", domain.FoldSearch(" Crème Brûlée "))
	assert.Equal(t, "garcon a l'aioli", domain.FoldSearch("Garçon à l'aïoli"))
}
