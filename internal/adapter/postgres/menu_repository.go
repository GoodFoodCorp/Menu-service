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

const itemCols = `m.id, m.tenant_id, m.name, m.description, m.price_cents, m.category, m.emoji, m.rating, m.available, m.sort_order, m.ingredients, m.image_data_url`

// nullableTenant converts the domain's "" (global) into a real SQL NULL —
// "" is not a valid UUID, so it can never collide with a real tenant id.
func nullableTenant(scope string) *string {
	if scope == "" {
		return nil
	}
	return &scope
}

// nullableString stores an empty string as SQL NULL — used for optional
// text columns like image_data_url, where "" and "no photo" mean the same
// thing.
func nullableString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// ListAvailableByTenant merges a restaurant's own available dishes with the
// global (head-office) catalog, excluding whatever that restaurant hid for
// itself — the customer-facing view.
func (r *MenuRepository) ListAvailableByTenant(ctx context.Context, tenantID string) ([]domain.MenuItem, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT `+itemCols+` FROM menu_items m
		 WHERE (m.tenant_id = $1 OR m.tenant_id IS NULL) AND m.available = TRUE
		   AND NOT EXISTS (
		     SELECT 1 FROM menu_item_tenant_overrides o
		     WHERE o.menu_item_id = m.id AND o.tenant_id = $1
		   )
		 ORDER BY m.sort_order ASC`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanItems(rows)
}

// ListByScope returns every dish of a scope (a restaurant, or "" for the
// global catalog), incl. unavailable — the manager/admin view of their own
// items.
func (r *MenuRepository) ListByScope(ctx context.Context, scope string) ([]domain.MenuItem, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT `+itemCols+` FROM menu_items m
		 WHERE m.tenant_id IS NOT DISTINCT FROM $1 ORDER BY m.sort_order ASC`, nullableTenant(scope))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanItems(rows)
}

// ListGlobalWithOverridesFor returns the global catalog, each item annotated
// with whether the given restaurant has hidden it for itself.
func (r *MenuRepository) ListGlobalWithOverridesFor(ctx context.Context, tenantID string) ([]domain.MenuItem, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT `+itemCols+`, (o.menu_item_id IS NOT NULL) AS hidden_for_viewer
		 FROM menu_items m
		 LEFT JOIN menu_item_tenant_overrides o ON o.menu_item_id = m.id AND o.tenant_id = $1
		 WHERE m.tenant_id IS NULL
		 ORDER BY m.sort_order ASC`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := []domain.MenuItem{}
	for rows.Next() {
		m, hiddenForViewer, err := scanItemWithOverride(rows)
		if err != nil {
			return nil, err
		}
		m.HiddenForViewer = hiddenForViewer
		items = append(items, *m)
	}
	return items, rows.Err()
}

func (r *MenuRepository) GetByID(ctx context.Context, id string) (*domain.MenuItem, error) {
	row := r.pool.QueryRow(ctx, `SELECT `+itemCols+` FROM menu_items m WHERE m.id = $1`, id)
	m, err := scanItem(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.NewNotFoundError("menu item not found")
	}
	return m, err
}

func (r *MenuRepository) GetByIDs(ctx context.Context, ids []string) ([]domain.MenuItem, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	rows, err := r.pool.Query(ctx, `SELECT `+itemCols+` FROM menu_items m WHERE m.id = ANY($1)`, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanItems(rows)
}

func (r *MenuRepository) Create(ctx context.Context, m *domain.MenuItem) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO menu_items (id, tenant_id, name, description, price_cents, category, emoji, rating, available, sort_order, ingredients, image_data_url)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`,
		m.ID, nullableTenant(m.TenantID), m.Name, m.Description, m.PriceCents,
		m.Category, m.Emoji, m.Rating, m.Available, m.SortOrder, m.Ingredients, nullableString(m.ImageDataURL))
	return err
}

func (r *MenuRepository) Update(ctx context.Context, m *domain.MenuItem) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE menu_items SET name=$1, description=$2, price_cents=$3, category=$4,
		 emoji=$5, available=$6, ingredients=$7, image_data_url=$8 WHERE id=$9`,
		m.Name, m.Description, m.PriceCents, m.Category, m.Emoji, m.Available, m.Ingredients,
		nullableString(m.ImageDataURL), m.ID)
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

func (r *MenuRepository) MaxSortOrder(ctx context.Context, scope string) (int, error) {
	var n int
	err := r.pool.QueryRow(ctx,
		`SELECT COALESCE(MAX(sort_order), 0) FROM menu_items WHERE tenant_id IS NOT DISTINCT FROM $1`,
		nullableTenant(scope)).Scan(&n)
	return n, err
}

// ToggleTenantOverride hides/shows a global item for one restaurant only.
func (r *MenuRepository) ToggleTenantOverride(ctx context.Context, tenantID, itemID string) (bool, error) {
	tag, err := r.pool.Exec(ctx,
		`DELETE FROM menu_item_tenant_overrides WHERE tenant_id = $1 AND menu_item_id = $2`,
		tenantID, itemID)
	if err != nil {
		return false, err
	}
	if tag.RowsAffected() > 0 {
		return false, nil // was hidden, now shown again
	}
	if _, err := r.pool.Exec(ctx,
		`INSERT INTO menu_item_tenant_overrides (tenant_id, menu_item_id) VALUES ($1,$2)`,
		tenantID, itemID); err != nil {
		return false, err
	}
	return true, nil // now hidden
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanItem(row rowScanner) (*domain.MenuItem, error) {
	var m domain.MenuItem
	var tenantID, imageDataURL *string
	if err := row.Scan(&m.ID, &tenantID, &m.Name, &m.Description, &m.PriceCents,
		&m.Category, &m.Emoji, &m.Rating, &m.Available, &m.SortOrder, &m.Ingredients, &imageDataURL); err != nil {
		return nil, err
	}
	if tenantID != nil {
		m.TenantID = *tenantID
	}
	if imageDataURL != nil {
		m.ImageDataURL = *imageDataURL
	}
	return &m, nil
}

func scanItemWithOverride(row rowScanner) (*domain.MenuItem, bool, error) {
	var m domain.MenuItem
	var tenantID, imageDataURL *string
	var hidden bool
	if err := row.Scan(&m.ID, &tenantID, &m.Name, &m.Description, &m.PriceCents,
		&m.Category, &m.Emoji, &m.Rating, &m.Available, &m.SortOrder, &m.Ingredients, &imageDataURL, &hidden); err != nil {
		return nil, false, err
	}
	if tenantID != nil {
		m.TenantID = *tenantID
	}
	if imageDataURL != nil {
		m.ImageDataURL = *imageDataURL
	}
	return &m, hidden, nil
}

func scanItems(rows pgx.Rows) ([]domain.MenuItem, error) {
	items := []domain.MenuItem{}
	for rows.Next() {
		m, err := scanItem(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, *m)
	}
	return items, rows.Err()
}
