package postgres

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	"goodfood/menu-service/internal/domain"
)

type RecipeRepository struct {
	pool *pgxpool.Pool
}

func NewRecipeRepository(pool *pgxpool.Pool) *RecipeRepository {
	return &RecipeRepository{pool: pool}
}

const recipeCols = `menu_item_id, stock_item_id, stock_item_name, unit, quantity::float8`

// Get returns a dish's recipe for one restaurant, ordered by ingredient name.
func (r *RecipeRepository) Get(ctx context.Context, tenantID, itemID string) ([]domain.RecipeLine, error) {
	byItem, err := r.Lookup(ctx, tenantID, []string{itemID})
	if err != nil {
		return nil, err
	}
	if lines, ok := byItem[itemID]; ok {
		return lines, nil
	}
	return []domain.RecipeLine{}, nil
}

// Replace swaps a dish's recipe for one restaurant in a single transaction;
// an empty list just deletes it.
func (r *RecipeRepository) Replace(ctx context.Context, tenantID, itemID string, lines []domain.RecipeLine) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op after commit

	if _, err := tx.Exec(ctx,
		`DELETE FROM menu_item_recipes WHERE tenant_id = $1 AND menu_item_id = $2`, tenantID, itemID); err != nil {
		return err
	}
	for _, l := range lines {
		if _, err := tx.Exec(ctx,
			`INSERT INTO menu_item_recipes (tenant_id, menu_item_id, stock_item_id, stock_item_name, unit, quantity)
			 VALUES ($1,$2,$3,$4,$5,$6)`,
			tenantID, itemID, l.StockItemID, l.StockItemName, l.Unit, l.Quantity); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

// Lookup returns the recipes of several dishes for one restaurant, keyed by
// dish id; dishes without a recipe are absent.
func (r *RecipeRepository) Lookup(ctx context.Context, tenantID string, itemIDs []string) (map[string][]domain.RecipeLine, error) {
	out := map[string][]domain.RecipeLine{}
	if len(itemIDs) == 0 {
		return out, nil
	}
	rows, err := r.pool.Query(ctx,
		`SELECT `+recipeCols+` FROM menu_item_recipes
		 WHERE tenant_id = $1 AND menu_item_id = ANY($2::text[]::uuid[])
		 ORDER BY menu_item_id, stock_item_name, stock_item_id`, tenantID, itemIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var itemID string
		var l domain.RecipeLine
		if err := rows.Scan(&itemID, &l.StockItemID, &l.StockItemName, &l.Unit, &l.Quantity); err != nil {
			return nil, err
		}
		out[itemID] = append(out[itemID], l)
	}
	return out, rows.Err()
}

// DishesUsingStockItems returns the distinct names of the dishes whose recipe
// (in this restaurant) uses each stock item, keyed by stock item id.
func (r *RecipeRepository) DishesUsingStockItems(ctx context.Context, tenantID string, stockItemIDs []string) (map[string][]string, error) {
	out := map[string][]string{}
	if len(stockItemIDs) == 0 {
		return out, nil
	}
	rows, err := r.pool.Query(ctx,
		`SELECT rec.stock_item_id::text, mi.name
		 FROM menu_item_recipes rec
		 JOIN menu_items mi ON mi.id = rec.menu_item_id
		 WHERE rec.tenant_id = $1 AND rec.stock_item_id = ANY($2::text[]::uuid[])
		 ORDER BY rec.stock_item_id, mi.name`, tenantID, stockItemIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var stockItemID, dish string
		if err := rows.Scan(&stockItemID, &dish); err != nil {
			return nil, err
		}
		out[stockItemID] = append(out[stockItemID], dish)
	}
	return out, rows.Err()
}

var _ domain.RecipeRepository = (*RecipeRepository)(nil)
