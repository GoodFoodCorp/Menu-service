package application

import (
	"context"

	"goodfood/menu-service/internal/domain"
)

// StarterMenu is the common base catalog every new restaurant starts with.
// Franchisees then customize it (add/edit/remove) for their own restaurant.
var StarterMenu = []domain.MenuItemInput{
	{Name: "Burger Deluxe", Description: "Steak haché, cheddar, bacon, oignons caramélisés", PriceCents: 1299, Category: "Burgers", Emoji: "🍔", Available: true},
	{Name: "Cheeseburger Classic", Description: "Steak, double cheddar, cornichons, sauce maison", PriceCents: 1099, Category: "Burgers", Emoji: "🍔", Available: true},
	{Name: "Pizza Margherita", Description: "Mozzarella, tomates fraîches, basilic", PriceCents: 1499, Category: "Pizzas", Emoji: "🍕", Available: true},
	{Name: "Pizza Regina", Description: "Jambon, champignons, mozzarella, olives", PriceCents: 1599, Category: "Pizzas", Emoji: "🍕", Available: true},
	{Name: "Salade César", Description: "Poulet grillé, parmesan, croûtons, sauce César", PriceCents: 999, Category: "Salades", Emoji: "🥗", Available: true},
	{Name: "Fondant au Chocolat", Description: "Cœur coulant, glace vanille", PriceCents: 699, Category: "Desserts", Emoji: "🍫", Available: true},
}

// SeedRestaurantMenus creates the starter menu for every restaurant that has
// none yet (idempotent). Each item is a distinct row owned by its restaurant,
// so restaurants never share menu data.
func (uc *UseCases) SeedRestaurantMenus(ctx context.Context, tenantIDs []string) (int, error) {
	created := 0
	for _, tenantID := range tenantIDs {
		count, err := uc.menu.CountByTenant(ctx, tenantID)
		if err != nil {
			return created, err
		}
		if count > 0 {
			continue
		}
		for i, in := range StarterMenu {
			item, err := domain.NewMenuItem(tenantID, in, i+1)
			if err != nil {
				return created, err
			}
			if err := uc.menu.Create(ctx, item); err != nil {
				return created, err
			}
			created++
		}
	}
	return created, nil
}
