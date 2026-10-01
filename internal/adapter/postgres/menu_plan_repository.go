package postgres

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"goodfood/menu-service/internal/domain"
)

type MenuPlanRepository struct {
	pool *pgxpool.Pool
}

func NewMenuPlanRepository(pool *pgxpool.Pool) *MenuPlanRepository {
	return &MenuPlanRepository{pool: pool}
}

const planSelect = `
	SELECT mp.id, mp.tenant_id, mp.name, mp.description, mp.price_cents, mp.emoji, mp.available, mp.sort_order, mp.image_data_url,
	       COALESCE(array_agg(mpd.menu_item_id::text ORDER BY mpd.sort_order) FILTER (WHERE mpd.menu_item_id IS NOT NULL), '{}')
	FROM menu_plans mp
	LEFT JOIN menu_plan_dishes mpd ON mpd.menu_plan_id = mp.id`

func (r *MenuPlanRepository) ListAvailableByTenant(ctx context.Context, tenantID string) ([]domain.MenuPlan, error) {
	rows, err := r.pool.Query(ctx,
		planSelect+`
		 WHERE (mp.tenant_id = $1 OR mp.tenant_id IS NULL) AND mp.available = TRUE
		   AND NOT EXISTS (
		     SELECT 1 FROM menu_plan_tenant_overrides o
		     WHERE o.menu_plan_id = mp.id AND o.tenant_id = $1
		   )
		 GROUP BY mp.id ORDER BY mp.sort_order ASC`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanPlans(rows)
}

func (r *MenuPlanRepository) ListByScope(ctx context.Context, scope string) ([]domain.MenuPlan, error) {
	rows, err := r.pool.Query(ctx,
		planSelect+`
		 WHERE mp.tenant_id IS NOT DISTINCT FROM $1
		 GROUP BY mp.id ORDER BY mp.sort_order ASC`, nullableTenant(scope))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanPlans(rows)
}

func (r *MenuPlanRepository) ListGlobalWithOverridesFor(ctx context.Context, tenantID string) ([]domain.MenuPlan, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT mp.id, mp.tenant_id, mp.name, mp.description, mp.price_cents, mp.emoji, mp.available, mp.sort_order, mp.image_data_url,
		        COALESCE(array_agg(DISTINCT mpd.menu_item_id::text) FILTER (WHERE mpd.menu_item_id IS NOT NULL), '{}'),
		        bool_or(o.menu_plan_id IS NOT NULL)
		 FROM menu_plans mp
		 LEFT JOIN menu_plan_dishes mpd ON mpd.menu_plan_id = mp.id
		 LEFT JOIN menu_plan_tenant_overrides o ON o.menu_plan_id = mp.id AND o.tenant_id = $1
		 WHERE mp.tenant_id IS NULL
		 GROUP BY mp.id ORDER BY mp.sort_order ASC`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	plans := []domain.MenuPlan{}
	for rows.Next() {
		var p domain.MenuPlan
		var tenantID, imageDataURL *string
		var hidden bool
		if err := rows.Scan(&p.ID, &tenantID, &p.Name, &p.Description, &p.PriceCents,
			&p.Emoji, &p.Available, &p.SortOrder, &imageDataURL, &p.DishIDs, &hidden); err != nil {
			return nil, err
		}
		if tenantID != nil {
			p.TenantID = *tenantID
		}
		if imageDataURL != nil {
			p.ImageDataURL = *imageDataURL
		}
		p.HiddenForViewer = hidden
		plans = append(plans, p)
	}
	return plans, rows.Err()
}

func (r *MenuPlanRepository) GetByID(ctx context.Context, id string) (*domain.MenuPlan, error) {
	rows, err := r.pool.Query(ctx, planSelect+` WHERE mp.id = $1 GROUP BY mp.id`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	plans, err := scanPlans(rows)
	if err != nil {
		return nil, err
	}
	if len(plans) == 0 {
		return nil, domain.NewNotFoundError("menu not found")
	}
	return &plans[0], nil
}

func (r *MenuPlanRepository) Create(ctx context.Context, p *domain.MenuPlan) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx,
		`INSERT INTO menu_plans (id, tenant_id, name, description, price_cents, emoji, available, sort_order, image_data_url)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
		p.ID, nullableTenant(p.TenantID), p.Name, p.Description, p.PriceCents, p.Emoji, p.Available, p.SortOrder,
		nullableString(p.ImageDataURL),
	); err != nil {
		return err
	}
	if err := insertDishes(ctx, tx, p.ID, p.DishIDs); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r *MenuPlanRepository) Update(ctx context.Context, p *domain.MenuPlan) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	tag, err := tx.Exec(ctx,
		`UPDATE menu_plans SET name=$1, description=$2, price_cents=$3, emoji=$4, available=$5, image_data_url=$6 WHERE id=$7`,
		p.Name, p.Description, p.PriceCents, p.Emoji, p.Available, nullableString(p.ImageDataURL), p.ID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.NewNotFoundError("menu not found")
	}
	if _, err := tx.Exec(ctx, `DELETE FROM menu_plan_dishes WHERE menu_plan_id = $1`, p.ID); err != nil {
		return err
	}
	if err := insertDishes(ctx, tx, p.ID, p.DishIDs); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func insertDishes(ctx context.Context, tx pgx.Tx, planID string, dishIDs []string) error {
	for i, dishID := range dishIDs {
		if _, err := tx.Exec(ctx,
			`INSERT INTO menu_plan_dishes (menu_plan_id, menu_item_id, sort_order) VALUES ($1,$2,$3)`,
			planID, dishID, i); err != nil {
			return err
		}
	}
	return nil
}

func (r *MenuPlanRepository) Delete(ctx context.Context, id string) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM menu_plans WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.NewNotFoundError("menu not found")
	}
	return nil
}

func (r *MenuPlanRepository) MaxSortOrder(ctx context.Context, scope string) (int, error) {
	var n int
	err := r.pool.QueryRow(ctx,
		`SELECT COALESCE(MAX(sort_order), 0) FROM menu_plans WHERE tenant_id IS NOT DISTINCT FROM $1`,
		nullableTenant(scope)).Scan(&n)
	return n, err
}

// ToggleTenantOverride hides/shows a global menu for one restaurant only.
func (r *MenuPlanRepository) ToggleTenantOverride(ctx context.Context, tenantID, planID string) (bool, error) {
	tag, err := r.pool.Exec(ctx,
		`DELETE FROM menu_plan_tenant_overrides WHERE tenant_id = $1 AND menu_plan_id = $2`,
		tenantID, planID)
	if err != nil {
		return false, err
	}
	if tag.RowsAffected() > 0 {
		return false, nil
	}
	if _, err := r.pool.Exec(ctx,
		`INSERT INTO menu_plan_tenant_overrides (tenant_id, menu_plan_id) VALUES ($1,$2)`,
		tenantID, planID); err != nil {
		return false, err
	}
	return true, nil
}

func scanPlans(rows pgx.Rows) ([]domain.MenuPlan, error) {
	plans := []domain.MenuPlan{}
	for rows.Next() {
		var p domain.MenuPlan
		var tenantID, imageDataURL *string
		if err := rows.Scan(&p.ID, &tenantID, &p.Name, &p.Description, &p.PriceCents,
			&p.Emoji, &p.Available, &p.SortOrder, &imageDataURL, &p.DishIDs); err != nil {
			return nil, err
		}
		if tenantID != nil {
			p.TenantID = *tenantID
		}
		if imageDataURL != nil {
			p.ImageDataURL = *imageDataURL
		}
		plans = append(plans, p)
	}
	return plans, rows.Err()
}
