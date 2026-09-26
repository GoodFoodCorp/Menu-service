package application

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"goodfood/menu-service/internal/domain"
)

// In-memory fake repository (no Postgres).
type fakeRepo struct {
	items map[string]*domain.MenuItem
}

func newFakeRepo() *fakeRepo { return &fakeRepo{items: map[string]*domain.MenuItem{}} }

func (f *fakeRepo) ListAvailableByTenant(_ context.Context, tenantID string) ([]domain.MenuItem, error) {
	out := []domain.MenuItem{}
	for _, m := range f.items {
		if m.TenantID == tenantID && m.Available {
			out = append(out, *m)
		}
	}
	return out, nil
}

func (f *fakeRepo) ListByTenant(_ context.Context, tenantID string) ([]domain.MenuItem, error) {
	out := []domain.MenuItem{}
	for _, m := range f.items {
		if m.TenantID == tenantID {
			out = append(out, *m)
		}
	}
	return out, nil
}

func (f *fakeRepo) GetByID(_ context.Context, id string) (*domain.MenuItem, error) {
	m, ok := f.items[id]
	if !ok {
		return nil, domain.NewNotFoundError("menu item not found")
	}
	cp := *m
	return &cp, nil
}

func (f *fakeRepo) ListByIDs(_ context.Context, ids []string) ([]domain.MenuItem, error) {
	out := []domain.MenuItem{}
	for _, id := range ids {
		if m, ok := f.items[id]; ok {
			out = append(out, *m)
		}
	}
	return out, nil
}

func (f *fakeRepo) Create(_ context.Context, m *domain.MenuItem) error {
	cp := *m
	f.items[m.ID] = &cp
	return nil
}

func (f *fakeRepo) Update(_ context.Context, m *domain.MenuItem) error {
	f.items[m.ID] = m
	return nil
}

func (f *fakeRepo) Delete(_ context.Context, id string) error {
	delete(f.items, id)
	return nil
}

func (f *fakeRepo) CountByTenant(_ context.Context, tenantID string) (int, error) {
	n := 0
	for _, m := range f.items {
		if m.TenantID == tenantID {
			n++
		}
	}
	return n, nil
}

func (f *fakeRepo) MaxSortOrder(_ context.Context, tenantID string) (int, error) {
	max := 0
	for _, m := range f.items {
		if m.TenantID == tenantID && m.SortOrder > max {
			max = m.SortOrder
		}
	}
	return max, nil
}

const (
	restaurantA = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
	restaurantB = "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
)

var (
	managerA = Actor{UserID: "mgr-a", TenantID: restaurantA, RoleSlugs: []string{RoleManager}}
	managerB = Actor{UserID: "mgr-b", TenantID: restaurantB, RoleSlugs: []string{RoleManager}}
	customer = Actor{UserID: "cust", RoleSlugs: []string{"user"}}
)

func validInput(name string) domain.MenuItemInput {
	return domain.MenuItemInput{Name: name, Category: "Burgers", PriceCents: 1200, Available: true}
}

func TestCreateItemScopedToOwnRestaurant(t *testing.T) {
	uc := NewUseCases(newFakeRepo())

	item, err := uc.CreateItem(context.Background(), managerA, validInput("Burger A"))
	require.NoError(t, err)
	assert.Equal(t, restaurantA, item.TenantID)
	assert.Equal(t, 1, item.SortOrder)
}

func TestNonManagerCannotCreate(t *testing.T) {
	uc := NewUseCases(newFakeRepo())
	_, err := uc.CreateItem(context.Background(), customer, validInput("x"))
	var derr *domain.Error
	require.ErrorAs(t, err, &derr)
	assert.Equal(t, domain.ErrCodeForbidden, derr.Code)
}

func TestManagerWithoutTenantRefused(t *testing.T) {
	uc := NewUseCases(newFakeRepo())
	noTenant := Actor{UserID: "m", RoleSlugs: []string{RoleManager}}
	_, err := uc.CreateItem(context.Background(), noTenant, validInput("x"))
	var derr *domain.Error
	require.ErrorAs(t, err, &derr)
	assert.Equal(t, domain.ErrCodeForbidden, derr.Code)
}

func TestTenantIsolation_ListAndEdit(t *testing.T) {
	uc := NewUseCases(newFakeRepo())

	itemA, _ := uc.CreateItem(context.Background(), managerA, validInput("Only A"))
	_, _ = uc.CreateItem(context.Background(), managerB, validInput("Only B"))

	// Manager A sees only their restaurant's items.
	mineA, err := uc.ListMyMenu(context.Background(), managerA)
	require.NoError(t, err)
	assert.Len(t, mineA, 1)
	assert.Equal(t, "Only A", mineA[0].Name)

	// Public browsing of restaurant B never returns restaurant A's items.
	publicB, err := uc.ListRestaurantMenu(context.Background(), restaurantB)
	require.NoError(t, err)
	require.Len(t, publicB, 1)
	assert.Equal(t, "Only B", publicB[0].Name)

	// Manager B cannot edit restaurant A's item.
	_, err = uc.UpdateItem(context.Background(), managerB, itemA.ID, validInput("hacked"))
	var derr *domain.Error
	require.ErrorAs(t, err, &derr)
	assert.Equal(t, domain.ErrCodeForbidden, derr.Code)

	// Manager B cannot delete restaurant A's item either.
	err = uc.DeleteItem(context.Background(), managerB, itemA.ID)
	require.ErrorAs(t, err, &derr)
	assert.Equal(t, domain.ErrCodeForbidden, derr.Code)
}

func TestUpdateAndDeleteOwnItem(t *testing.T) {
	uc := NewUseCases(newFakeRepo())
	item, _ := uc.CreateItem(context.Background(), managerA, validInput("Burger"))

	updated, err := uc.UpdateItem(context.Background(), managerA, item.ID,
		domain.MenuItemInput{Name: "Burger XL", Category: "Burgers", PriceCents: 1500, Available: false})
	require.NoError(t, err)
	assert.Equal(t, "Burger XL", updated.Name)
	assert.False(t, updated.Available)

	// Unavailable items disappear from the public menu but stay in manage view.
	public, _ := uc.ListRestaurantMenu(context.Background(), restaurantA)
	assert.Empty(t, public)
	mine, _ := uc.ListMyMenu(context.Background(), managerA)
	assert.Len(t, mine, 1)

	require.NoError(t, uc.DeleteItem(context.Background(), managerA, item.ID))
	mine, _ = uc.ListMyMenu(context.Background(), managerA)
	assert.Empty(t, mine)
}

func TestPublicMenuRequiresRestaurantId(t *testing.T) {
	uc := NewUseCases(newFakeRepo())
	_, err := uc.ListRestaurantMenu(context.Background(), "")
	var derr *domain.Error
	require.ErrorAs(t, err, &derr)
	assert.Equal(t, domain.ErrCodeValidation, derr.Code)
}

func TestSeedRestaurantMenusIdempotent(t *testing.T) {
	uc := NewUseCases(newFakeRepo())

	created, err := uc.SeedRestaurantMenus(context.Background(), []string{restaurantA, restaurantB})
	require.NoError(t, err)
	assert.Equal(t, 2*len(StarterMenu), created)

	// Second run creates nothing (idempotent).
	created2, err := uc.SeedRestaurantMenus(context.Background(), []string{restaurantA, restaurantB})
	require.NoError(t, err)
	assert.Equal(t, 0, created2)
}
