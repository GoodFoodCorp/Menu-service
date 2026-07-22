package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"goodfood/menu-service/internal/domain"
)

type MenuRepository struct {
	pool *pgxpool.Pool
}

func NewMenuRepository(pool *pgxpool.Pool) *MenuRepository {
	return &MenuRepository{pool: pool}
}

const selectCols = `id, tenant_id, name, description, price_cents, category, emoji, rating, available, sort_order`

func (r *MenuRepository) ListAvailableByTenant(ctx context.Context, tenantID string) ([]domain.MenuItem, error) {
	return r.query(ctx,
		`SELECT `+selectCols+` FROM menu_items
		 WHERE tenant_id = $1 AND available = TRUE ORDER BY sort_order ASC`, tenantID)
}

func (r *MenuRepository) ListByTenant(ctx context.Context, tenantID string) ([]domain.MenuItem, error) {
	return r.query(ctx,
		`SELECT `+selectCols+` FROM menu_items
		 WHERE tenant_id = $1 ORDER BY sort_order ASC`, tenantID)
}

func (r *MenuRepository) GetByID(ctx context.Context, id string) (*domain.MenuItem, error) {
	var m domain.MenuItem
	err := r.pool.QueryRow(ctx, `SELECT `+selectCols+` FROM menu_items WHERE id = $1`, id).
		Scan(&m.ID, &m.TenantID, &m.Name, &m.Description, &m.PriceCents,
			&m.Category, &m.Emoji, &m.Rating, &m.Available, &m.SortOrder)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.NewNotFoundError("menu item not found")
	}
	if err != nil {
		return nil, err
	}
	return &m, nil
}

func (r *MenuRepository) Create(ctx context.Context, m *domain.MenuItem) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO menu_items (`+selectCols+`)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`,
		m.ID, m.TenantID, m.Name, m.Description, m.PriceCents,
		m.Category, m.Emoji, m.Rating, m.Available, m.SortOrder)
	return err
}

func (r *MenuRepository) Update(ctx context.Context, m *domain.MenuItem) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE menu_items SET name=$1, description=$2, price_cents=$3, category=$4,
		 emoji=$5, available=$6 WHERE id=$7`,
		m.Name, m.Description, m.PriceCents, m.Category, m.Emoji, m.Available, m.ID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.NewNotFoundError("menu item not found")
	}
	return nil
}

func (r *MenuRepository) Delete(ctx context.Context, id string) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM menu_items WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.NewNotFoundError("menu item not found")
	}
	return nil
}

func (r *MenuRepository) CountByTenant(ctx context.Context, tenantID string) (int, error) {
	var n int
	err := r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM menu_items WHERE tenant_id = $1`, tenantID).Scan(&n)
	return n, err
}

func (r *MenuRepository) MaxSortOrder(ctx context.Context, tenantID string) (int, error) {
	var n int
	err := r.pool.QueryRow(ctx,
		`SELECT COALESCE(MAX(sort_order), 0) FROM menu_items WHERE tenant_id = $1`, tenantID).Scan(&n)
	return n, err
}

func (r *MenuRepository) query(ctx context.Context, sql, tenantID string) ([]domain.MenuItem, error) {
	rows, err := r.pool.Query(ctx, sql, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := []domain.MenuItem{}
	for rows.Next() {
		var m domain.MenuItem
		if err := rows.Scan(&m.ID, &m.TenantID, &m.Name, &m.Description, &m.PriceCents,
			&m.Category, &m.Emoji, &m.Rating, &m.Available, &m.SortOrder); err != nil {
			return nil, err
		}
		items = append(items, m)
	}
	return items, rows.Err()
}
