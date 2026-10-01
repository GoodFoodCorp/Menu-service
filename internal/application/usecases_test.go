package application

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"goodfood/menu-service/internal/domain"
)

// In-memory fake repositories (no Postgres). "" tenant means the global
// (head-office) scope, same convention as the real repository.
type fakeRepo struct {
	items     map[string]*domain.MenuItem
	overrides map[string]bool // key: tenantID+"|"+itemID
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{items: map[string]*domain.MenuItem{}, overrides: map[string]bool{}}
}

func (f *fakeRepo) ListAvailableByTenant(_ context.Context, tenantID string) ([]domain.MenuItem, error) {
	out := []domain.MenuItem{}
	for _, m := range f.items {
		if (m.TenantID == tenantID || m.IsGlobal()) && m.Available && !f.overrides[tenantID+"|"+m.ID] {
			out = append(out, *m)
		}
	}
	return out, nil
}

func (f *fakeRepo) ListGlobalWithOverridesFor(_ context.Context, tenantID string) ([]domain.MenuItem, error) {
	out := []domain.MenuItem{}
	for _, m := range f.items {
		if m.IsGlobal() {
			cp := *m
			cp.HiddenForViewer = f.overrides[tenantID+"|"+m.ID]
			out = append(out, cp)
		}
	}
	return out, nil
}

func (f *fakeRepo) ToggleTenantOverride(_ context.Context, tenantID, itemID string) (bool, error) {
	key := tenantID + "|" + itemID
	hidden := !f.overrides[key]
	if hidden {
		f.overrides[key] = true
	} else {
		delete(f.overrides, key)
	}
	return hidden, nil
}

func (f *fakeRepo) ListByScope(_ context.Context, scope string) ([]domain.MenuItem, error) {
	out := []domain.MenuItem{}
	for _, m := range f.items {
		if m.TenantID == scope {
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

func (f *fakeRepo) MaxSortOrder(_ context.Context, scope string) (int, error) {
	max := 0
	for _, m := range f.items {
		if m.TenantID == scope && m.SortOrder > max {
			max = m.SortOrder
		}
	}
	return max, nil
}

// Minimal fake for MenuPlanRepository — enough for the tests that exercise
// menu-plan creation/validation.
type fakePlanRepo struct {
	plans     map[string]*domain.MenuPlan
	overrides map[string]bool
}

func newFakePlanRepo() *fakePlanRepo {
	return &fakePlanRepo{plans: map[string]*domain.MenuPlan{}, overrides: map[string]bool{}}
}

func (f *fakePlanRepo) ListAvailableByTenant(_ context.Context, tenantID string) ([]domain.MenuPlan, error) {
	out := []domain.MenuPlan{}
	for _, p := range f.plans {
		if (p.TenantID == tenantID || p.TenantID == "") && p.Available && !f.overrides[tenantID+"|"+p.ID] {
			out = append(out, *p)
		}
	}
	return out, nil
}

func (f *fakePlanRepo) ListGlobalWithOverridesFor(_ context.Context, tenantID string) ([]domain.MenuPlan, error) {
	out := []domain.MenuPlan{}
	for _, p := range f.plans {
		if p.IsGlobal() {
			cp := *p
			cp.HiddenForViewer = f.overrides[tenantID+"|"+p.ID]
			out = append(out, cp)
		}
	}
	return out, nil
}

func (f *fakePlanRepo) ToggleTenantOverride(_ context.Context, tenantID, planID string) (bool, error) {
	key := tenantID + "|" + planID
	hidden := !f.overrides[key]
	if hidden {
		f.overrides[key] = true
	} else {
		delete(f.overrides, key)
	}
	return hidden, nil
}

func (f *fakePlanRepo) ListByScope(_ context.Context, scope string) ([]domain.MenuPlan, error) {
	out := []domain.MenuPlan{}
	for _, p := range f.plans {
		if p.TenantID == scope {
			out = append(out, *p)
		}
	}
	return out, nil
}

func (f *fakePlanRepo) GetByID(_ context.Context, id string) (*domain.MenuPlan, error) {
	p, ok := f.plans[id]
	if !ok {
		return nil, domain.NewNotFoundError("menu not found")
	}
	cp := *p
	return &cp, nil
}

func (f *fakePlanRepo) Create(_ context.Context, p *domain.MenuPlan) error {
	cp := *p
	f.plans[p.ID] = &cp
	return nil
}

func (f *fakePlanRepo) Update(_ context.Context, p *domain.MenuPlan) error {
	f.plans[p.ID] = p
	return nil
}

func (f *fakePlanRepo) Delete(_ context.Context, id string) error {
	delete(f.plans, id)
	return nil
}

func (f *fakePlanRepo) MaxSortOrder(_ context.Context, scope string) (int, error) {
	max := 0
	for _, p := range f.plans {
		if p.TenantID == scope && p.SortOrder > max {
			max = p.SortOrder
		}
	}
	return max, nil
}

// Minimal fake for CategoryRepository.
type fakeCategoryRepo struct {
	categories []domain.Category
}

func newFakeCategoryRepo() *fakeCategoryRepo { return &fakeCategoryRepo{} }

func (f *fakeCategoryRepo) List(_ context.Context) ([]domain.Category, error) {
	return f.categories, nil
}

func (f *fakeCategoryRepo) Create(_ context.Context, c *domain.Category) error {
	for _, existing := range f.categories {
		if existing.Name == c.Name {
			return domain.NewValidationError("this category already exists")
		}
	}
	f.categories = append(f.categories, *c)
	return nil
}

func (f *fakeCategoryRepo) Delete(_ context.Context, id string) error {
	for i, c := range f.categories {
		if c.ID == id {
			f.categories = append(f.categories[:i], f.categories[i+1:]...)
			return nil
		}
	}
	return domain.NewNotFoundError("category not found")
}

func (f *fakeCategoryRepo) MaxSortOrder(_ context.Context) (int, error) {
	max := 0
	for _, c := range f.categories {
		if c.SortOrder > max {
			max = c.SortOrder
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
	headOffice = Actor{UserID: "admin", RoleSlugs: []string{RoleAdmin}}
	customer   = Actor{UserID: "cust", RoleSlugs: []string{"user"}}
)

func newUC() *UseCases {
	return NewUseCases(newFakeRepo(), newFakePlanRepo(), newFakeCategoryRepo())
}

func validInput(name string) domain.MenuItemInput {
	return domain.MenuItemInput{Name: name, Category: "Burgers", PriceCents: 1200, Available: true}
}

func TestCreateItemScopedToOwnRestaurant(t *testing.T) {
	uc := newUC()

	item, err := uc.CreateItem(context.Background(), managerA, validInput("Burger A"))
	require.NoError(t, err)
	assert.Equal(t, restaurantA, item.TenantID)
	assert.Equal(t, 1, item.SortOrder)
}

func TestNonManagerCannotCreate(t *testing.T) {
	uc := newUC()
	_, err := uc.CreateItem(context.Background(), customer, validInput("x"))
	var derr *domain.Error
	require.ErrorAs(t, err, &derr)
	assert.Equal(t, domain.ErrCodeForbidden, derr.Code)
}

func TestManagerWithoutTenantRefused(t *testing.T) {
	uc := newUC()
	noTenant := Actor{UserID: "m", RoleSlugs: []string{RoleManager}}
	_, err := uc.CreateItem(context.Background(), noTenant, validInput("x"))
	var derr *domain.Error
	require.ErrorAs(t, err, &derr)
	assert.Equal(t, domain.ErrCodeForbidden, derr.Code)
}

func TestTenantIsolation_ListAndEdit(t *testing.T) {
	uc := newUC()

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
	uc := newUC()
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
	uc := newUC()
	_, err := uc.ListRestaurantMenu(context.Background(), "")
	var derr *domain.Error
	require.ErrorAs(t, err, &derr)
	assert.Equal(t, domain.ErrCodeValidation, derr.Code)
}

func TestSeedRestaurantMenusIdempotent(t *testing.T) {
	uc := newUC()

	created, err := uc.SeedRestaurantMenus(context.Background(), []string{restaurantA, restaurantB})
	require.NoError(t, err)
	assert.Equal(t, 2*len(StarterMenu), created)

	// Second run creates nothing (idempotent).
	created2, err := uc.SeedRestaurantMenus(context.Background(), []string{restaurantA, restaurantB})
	require.NoError(t, err)
	assert.Equal(t, 0, created2)
}

// ── Global (head-office) catalog ────────────────────────────────────────

func TestAdminCreatesGlobalItem(t *testing.T) {
	uc := newUC()
	item, err := uc.CreateItem(context.Background(), headOffice, validInput("Coca-Cola"))
	require.NoError(t, err)
	assert.Empty(t, item.TenantID)
	assert.True(t, item.IsGlobal())
}

func TestGlobalItemsAreVisibleToEveryRestaurant(t *testing.T) {
	uc := newUC()
	_, err := uc.CreateItem(context.Background(), headOffice, validInput("Coca-Cola"))
	require.NoError(t, err)
	_, err = uc.CreateItem(context.Background(), managerA, validInput("Only A"))
	require.NoError(t, err)

	publicA, err := uc.ListRestaurantMenu(context.Background(), restaurantA)
	require.NoError(t, err)
	assert.Len(t, publicA, 2) // "Only A" + the global "Coca-Cola"

	publicB, err := uc.ListRestaurantMenu(context.Background(), restaurantB)
	require.NoError(t, err)
	assert.Len(t, publicB, 1) // only the global "Coca-Cola" — B has nothing of its own
}

func TestFranchiseeCannotEditGlobalItem(t *testing.T) {
	uc := newUC()
	global, err := uc.CreateItem(context.Background(), headOffice, validInput("Coca-Cola"))
	require.NoError(t, err)

	_, err = uc.UpdateItem(context.Background(), managerA, global.ID, validInput("hacked"))
	var derr *domain.Error
	require.ErrorAs(t, err, &derr)
	assert.Equal(t, domain.ErrCodeForbidden, derr.Code)
}

func TestAdminCannotEditFranchiseeItem(t *testing.T) {
	uc := newUC()
	itemA, err := uc.CreateItem(context.Background(), managerA, validInput("Only A"))
	require.NoError(t, err)

	_, err = uc.UpdateItem(context.Background(), headOffice, itemA.ID, validInput("hacked"))
	var derr *domain.Error
	require.ErrorAs(t, err, &derr)
	assert.Equal(t, domain.ErrCodeForbidden, derr.Code)
}

func TestFranchiseeSeesGlobalItemsInManagementView(t *testing.T) {
	uc := newUC()
	_, err := uc.CreateItem(context.Background(), headOffice, validInput("Coca-Cola"))
	require.NoError(t, err)
	_, err = uc.CreateItem(context.Background(), managerA, validInput("Only A"))
	require.NoError(t, err)

	mine, err := uc.ListMyMenu(context.Background(), managerA)
	require.NoError(t, err)
	require.Len(t, mine, 2) // their own dish + the read-only global one
}

func TestFranchiseeCanHideGlobalItemForOwnRestaurantOnly(t *testing.T) {
	uc := newUC()
	global, err := uc.CreateItem(context.Background(), headOffice, validInput("Coca-Cola"))
	require.NoError(t, err)

	// Manager A hides it — for their restaurant only.
	updated, err := uc.ToggleItemAvailability(context.Background(), managerA, global.ID)
	require.NoError(t, err)
	assert.True(t, updated.HiddenForViewer)

	// The item itself is untouched: still available globally.
	stillGlobal, err := uc.ListMyMenu(context.Background(), headOffice)
	require.NoError(t, err)
	require.Len(t, stillGlobal, 1)
	assert.True(t, stillGlobal[0].Available)

	// Gone from restaurant A's public menu…
	publicA, err := uc.ListRestaurantMenu(context.Background(), restaurantA)
	require.NoError(t, err)
	assert.Empty(t, publicA)

	// …but still visible at restaurant B, which never hid it.
	publicB, err := uc.ListRestaurantMenu(context.Background(), restaurantB)
	require.NoError(t, err)
	assert.Len(t, publicB, 1)

	// Toggling again un-hides it for restaurant A.
	updated2, err := uc.ToggleItemAvailability(context.Background(), managerA, global.ID)
	require.NoError(t, err)
	assert.False(t, updated2.HiddenForViewer)
}

func TestFranchiseeCanStillFreelyToggleTheirOwnItem(t *testing.T) {
	uc := newUC()
	item, err := uc.CreateItem(context.Background(), managerA, validInput("Only A"))
	require.NoError(t, err)
	require.True(t, item.Available)

	updated, err := uc.ToggleItemAvailability(context.Background(), managerA, item.ID)
	require.NoError(t, err)
	assert.False(t, updated.Available)
	assert.False(t, updated.HiddenForViewer) // not the override path
}

func TestManagerCannotToggleAnotherRestaurantsItem(t *testing.T) {
	uc := newUC()
	itemB, err := uc.CreateItem(context.Background(), managerB, validInput("Only B"))
	require.NoError(t, err)

	_, err = uc.ToggleItemAvailability(context.Background(), managerA, itemB.ID)
	var derr *domain.Error
	require.ErrorAs(t, err, &derr)
	assert.Equal(t, domain.ErrCodeForbidden, derr.Code)
}

// ── Menu plans ("formules") ─────────────────────────────────────────────

func TestCreateMenuPlanFromOwnAndGlobalDishes(t *testing.T) {
	uc := newUC()
	burger, err := uc.CreateItem(context.Background(), managerA, validInput("Burger"))
	require.NoError(t, err)
	soda, err := uc.CreateItem(context.Background(), headOffice, validInput("Coca-Cola"))
	require.NoError(t, err)

	plan, err := uc.CreateMenuPlan(context.Background(), managerA, domain.MenuPlanInput{
		Name: "Menu Burger", PriceCents: 1500, Available: true,
		DishIDs: []string{burger.ID, soda.ID},
	})
	require.NoError(t, err)
	assert.Equal(t, restaurantA, plan.TenantID)
	assert.ElementsMatch(t, []string{burger.ID, soda.ID}, plan.DishIDs)
}

func TestCannotBuildMenuPlanFromAnotherRestaurantsDish(t *testing.T) {
	uc := newUC()
	itemB, err := uc.CreateItem(context.Background(), managerB, validInput("Only B"))
	require.NoError(t, err)

	_, err = uc.CreateMenuPlan(context.Background(), managerA, domain.MenuPlanInput{
		Name: "Menu illégal", PriceCents: 1000, Available: true,
		DishIDs: []string{itemB.ID},
	})
	var derr *domain.Error
	require.ErrorAs(t, err, &derr)
	assert.Equal(t, domain.ErrCodeValidation, derr.Code)
}

// ── Categories ───────────────────────────────────────────────────────────

func TestAdminCanCreateCategory(t *testing.T) {
	uc := newUC()
	cat, err := uc.CreateCategory(context.Background(), headOffice, "Tacos")
	require.NoError(t, err)
	assert.Equal(t, "Tacos", cat.Name)

	list, err := uc.ListCategories(context.Background())
	require.NoError(t, err)
	assert.Len(t, list, 1)
}

func TestFranchiseeCannotCreateCategory(t *testing.T) {
	uc := newUC()
	_, err := uc.CreateCategory(context.Background(), managerA, "Tacos")
	var derr *domain.Error
	require.ErrorAs(t, err, &derr)
	assert.Equal(t, domain.ErrCodeForbidden, derr.Code)
}

func TestCannotCreateDuplicateCategory(t *testing.T) {
	uc := newUC()
	_, err := uc.CreateCategory(context.Background(), headOffice, "Tacos")
	require.NoError(t, err)
	_, err = uc.CreateCategory(context.Background(), headOffice, "Tacos")
	var derr *domain.Error
	require.ErrorAs(t, err, &derr)
	assert.Equal(t, domain.ErrCodeValidation, derr.Code)
}

func TestFranchiseeCannotDeleteCategory(t *testing.T) {
	uc := newUC()
	cat, err := uc.CreateCategory(context.Background(), headOffice, "Tacos")
	require.NoError(t, err)

	err = uc.DeleteCategory(context.Background(), managerA, cat.ID)
	var derr *domain.Error
	require.ErrorAs(t, err, &derr)
	assert.Equal(t, domain.ErrCodeForbidden, derr.Code)
}

func TestMenuPlanRequiresAtLeastOneDish(t *testing.T) {
	uc := newUC()
	_, err := uc.CreateMenuPlan(context.Background(), managerA, domain.MenuPlanInput{
		Name: "Menu vide", PriceCents: 1000, Available: true, DishIDs: nil,
	})
	var derr *domain.Error
	require.ErrorAs(t, err, &derr)
	assert.Equal(t, domain.ErrCodeValidation, derr.Code)
}

// ── Photos ───────────────────────────────────────────────────────────────

func TestDishAcceptsAValidImage(t *testing.T) {
	uc := newUC()
	in := validInput("Burger avec photo")
	in.ImageDataURL = "data:image/jpeg;base64,/9j/4AAQSkZJRg=="
	item, err := uc.CreateItem(context.Background(), managerA, in)
	require.NoError(t, err)
	assert.Equal(t, in.ImageDataURL, item.ImageDataURL)
}

func TestDishRejectsANonImageDataURL(t *testing.T) {
	uc := newUC()
	in := validInput("Burger suspect")
	in.ImageDataURL = "data:text/html;base64,PHNjcmlwdD4="
	_, err := uc.CreateItem(context.Background(), managerA, in)
	var derr *domain.Error
	require.ErrorAs(t, err, &derr)
	assert.Equal(t, domain.ErrCodeValidation, derr.Code)
}

func TestDishRejectsAnOversizedImage(t *testing.T) {
	uc := newUC()
	in := validInput("Burger trop lourd")
	in.ImageDataURL = "data:image/jpeg;base64," + strings.Repeat("A", 5_000_000)
	_, err := uc.CreateItem(context.Background(), managerA, in)
	var derr *domain.Error
	require.ErrorAs(t, err, &derr)
	assert.Equal(t, domain.ErrCodeValidation, derr.Code)
}
