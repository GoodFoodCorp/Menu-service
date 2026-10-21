package http

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/rs/zerolog"

	"goodfood/menu-service/internal/application"
	"goodfood/menu-service/internal/domain"
)

const (
	testJWTSecret  = "test-jwt-secret"
	testServiceKey = "test-service-key"
	restaurantA    = "11111111-1111-1111-1111-111111111111"
	dishWithRecipe = "22222222-2222-2222-2222-222222222222"
	dishNoRecipe   = "33333333-3333-3333-3333-333333333333"
	stockFlour     = "44444444-4444-4444-4444-444444444444"
	stockEgg       = "55555555-5555-5555-5555-555555555555"
)

// Minimal local fakes: only what the recipe endpoints touch. The embedded
// interfaces make any unexpected call panic loudly.
type fakeMenuRepo struct{ domain.MenuRepository }

func (fakeMenuRepo) GetByID(_ context.Context, id string) (*domain.MenuItem, error) {
	if id == dishWithRecipe || id == dishNoRecipe {
		return &domain.MenuItem{ID: id, TenantID: restaurantA}, nil
	}
	return nil, domain.NewNotFoundError("menu item not found")
}

type fakeRecipeRepo struct {
	data map[string][]domain.RecipeLine
}

func (f *fakeRecipeRepo) Get(_ context.Context, tenantID, itemID string) ([]domain.RecipeLine, error) {
	return f.data[tenantID+"/"+itemID], nil
}
func (f *fakeRecipeRepo) Replace(_ context.Context, tenantID, itemID string, lines []domain.RecipeLine) error {
	if len(lines) == 0 {
		delete(f.data, tenantID+"/"+itemID)
		return nil
	}
	f.data[tenantID+"/"+itemID] = lines
	return nil
}
func (f *fakeRecipeRepo) Lookup(_ context.Context, tenantID string, itemIDs []string) (map[string][]domain.RecipeLine, error) {
	out := map[string][]domain.RecipeLine{}
	for _, id := range itemIDs {
		if l := f.data[tenantID+"/"+id]; len(l) > 0 {
			out[id] = l
		}
	}
	return out, nil
}

func newTestRouter() http.Handler {
	uc := application.NewUseCases(fakeMenuRepo{}, nil, nil, &fakeRecipeRepo{data: map[string][]domain.RecipeLine{}})
	return NewRouter(NewMenuHandler(uc), testJWTSecret, testServiceKey, zerolog.Nop(),
		func(context.Context) error { return nil })
}

func token(t *testing.T, role string) string {
	t.Helper()
	tok, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": "user-1", "tenant_id": restaurantA, "role_slugs": []string{role},
		"exp": time.Now().Add(time.Hour).Unix(),
	}).SignedString([]byte(testJWTSecret))
	if err != nil {
		t.Fatal(err)
	}
	return tok
}

func do(h http.Handler, method, path, body string, headers map[string]string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestPUT_recipe_without_token_is_401(t *testing.T) {
	rec := do(newTestRouter(), http.MethodPut, "/api/menu/"+dishWithRecipe+"/recipe", `{"lines":[]}`, nil)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d", rec.Code)
	}
}

func TestPUT_recipe_manager_then_GET_returns_same_lines(t *testing.T) {
	h := newTestRouter()
	auth := map[string]string{"Authorization": "Bearer " + token(t, "manager")}
	body := fmt.Sprintf(`{"lines":[{"stock_item_id":%q,"stock_item_name":"Farine","unit":"kg","quantity":0.25},{"stock_item_id":%q,"stock_item_name":"Oeuf","unit":"piece","quantity":2}]}`, stockFlour, stockEgg)

	put := do(h, http.MethodPut, "/api/menu/"+dishWithRecipe+"/recipe", body, auth)
	if put.Code != http.StatusOK {
		t.Fatalf("PUT: want 200, got %d: %s", put.Code, put.Body)
	}
	var saved struct {
		MenuItemID string `json:"menu_item_id"`
		Lines      []struct {
			StockItemID   string  `json:"stock_item_id"`
			StockItemName string  `json:"stock_item_name"`
			Unit          string  `json:"unit"`
			Quantity      float64 `json:"quantity"`
		} `json:"lines"`
	}
	if err := json.Unmarshal(put.Body.Bytes(), &saved); err != nil {
		t.Fatal(err)
	}
	if saved.MenuItemID != dishWithRecipe || len(saved.Lines) != 2 ||
		saved.Lines[0].StockItemID != stockFlour || saved.Lines[0].StockItemName != "Farine" ||
		saved.Lines[0].Unit != "kg" || saved.Lines[0].Quantity != 0.25 || saved.Lines[1].Quantity != 2 {
		t.Fatalf("PUT body does not echo the saved recipe: %s", put.Body)
	}
	if !strings.Contains(put.Body.String(), `"stock_item_id"`) || !strings.Contains(put.Body.String(), `"menu_item_id"`) {
		t.Fatalf("PUT body must use snake_case keys: %s", put.Body)
	}

	get := do(h, http.MethodGet, "/api/menu/"+dishWithRecipe+"/recipe", "", auth)
	if get.Code != http.StatusOK {
		t.Fatalf("GET: want 200, got %d: %s", get.Code, get.Body)
	}
	var got map[string]any
	if err := json.Unmarshal(get.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got["menu_item_id"] != dishWithRecipe {
		t.Errorf("menu_item_id = %v", got["menu_item_id"])
	}
	lines, _ := got["lines"].([]any)
	if len(lines) != 2 {
		t.Fatalf("want 2 lines, got %v", got["lines"])
	}
	first := lines[0].(map[string]any)
	want := map[string]any{"stock_item_id": stockFlour, "stock_item_name": "Farine", "unit": "kg", "quantity": 0.25}
	for k, v := range want {
		if first[k] != v {
			t.Errorf("line[0].%s = %v, want %v", k, first[k], v)
		}
	}

	// Dish without recipe: lines is an empty array, not null.
	empty := do(h, http.MethodGet, "/api/menu/"+dishNoRecipe+"/recipe", "", auth)
	if !strings.Contains(empty.Body.String(), `"lines":[]`) {
		t.Errorf("want empty lines array, got %s", empty.Body)
	}
}

func TestRecipe_admin_is_403_and_invalid_quantity_is_400(t *testing.T) {
	h := newTestRouter()
	admin := map[string]string{"Authorization": "Bearer " + token(t, "admin")}
	if rec := do(h, http.MethodGet, "/api/menu/"+dishWithRecipe+"/recipe", "", admin); rec.Code != http.StatusForbidden {
		t.Errorf("admin GET: want 403, got %d", rec.Code)
	}
	mgr := map[string]string{"Authorization": "Bearer " + token(t, "manager")}
	bad := fmt.Sprintf(`{"lines":[{"stock_item_id":%q,"quantity":0}]}`, stockFlour)
	if rec := do(h, http.MethodPut, "/api/menu/"+dishWithRecipe+"/recipe", bad, mgr); rec.Code != http.StatusBadRequest {
		t.Errorf("zero quantity: want 400, got %d", rec.Code)
	}
	if rec := do(h, http.MethodPut, "/api/menu/"+dishWithRecipe+"/recipe", `{nope`, mgr); rec.Code != http.StatusBadRequest {
		t.Errorf("bad json: want 400, got %d", rec.Code)
	}
}

func TestPUT_recipe_oversized_body_is_413(t *testing.T) {
	mgr := map[string]string{"Authorization": "Bearer " + token(t, "manager")}
	big := fmt.Sprintf(`{"lines":[{"stock_item_id":%q,"stock_item_name":%q,"quantity":1}]}`, stockFlour, strings.Repeat("a", maxBodyBytes+10))
	rec := do(newTestRouter(), http.MethodPut, "/api/menu/"+dishWithRecipe+"/recipe", big, mgr)
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("want 413, got %d", rec.Code)
	}
}

func TestInternal_lookup_oversized_body_is_413(t *testing.T) {
	big := lookupBody(restaurantA, strings.Repeat("a", maxBodyBytes+10))
	rec := do(newTestRouter(), http.MethodPost, "/internal/recipes/lookup", big, map[string]string{"X-Service-Key": testServiceKey})
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("want 413, got %d", rec.Code)
	}
}

func TestPUT_recipe_strict_body_decoding(t *testing.T) {
	h := newTestRouter()
	seedRecipe(t, h)
	mgr := map[string]string{"Authorization": "Bearer " + token(t, "manager")}
	path := "/api/menu/" + dishWithRecipe + "/recipe"
	for name, body := range map[string]string{
		"misspelled lines": `{"line":[]}`,
		"unknown field":    `{"lines":[],"extra":1}`,
		"empty object":     `{}`,
		"null lines":       `{"lines":null}`,
		"trailing garbage": `{"lines":[]} garbage`,
		"second object":    `{"lines":[]}{"lines":[]}`,
	} {
		if rec := do(h, http.MethodPut, path, body, mgr); rec.Code != http.StatusBadRequest {
			t.Errorf("%s: want 400, got %d", name, rec.Code)
		}
	}
	// None of the rejected requests may have deleted the recipe.
	get := do(h, http.MethodGet, path, "", mgr)
	if !strings.Contains(get.Body.String(), stockFlour) {
		t.Fatalf("recipe was lost after rejected PUTs: %s", get.Body)
	}
	// An explicit empty array still deletes it.
	if rec := do(h, http.MethodPut, path, `{"lines":[]}`, mgr); rec.Code != http.StatusOK {
		t.Fatalf("delete: want 200, got %d", rec.Code)
	}
	if get := do(h, http.MethodGet, path, "", mgr); !strings.Contains(get.Body.String(), `"lines":[]`) {
		t.Fatalf("want empty lines after delete, got %s", get.Body)
	}
}

func TestInternal_lookup_strict_body_decoding(t *testing.T) {
	h := newTestRouter()
	key := map[string]string{"X-Service-Key": testServiceKey}
	for name, body := range map[string]string{
		"unknown field":         fmt.Sprintf(`{"restaurant_id":%q,"menu_item_ids":[],"x":1}`, restaurantA),
		"misspelled ids":        fmt.Sprintf(`{"restaurant_id":%q,"menu_item_id":[%q]}`, restaurantA, dishWithRecipe),
		"missing restaurant_id": fmt.Sprintf(`{"menu_item_ids":[%q]}`, dishWithRecipe),
		"null menu_item_ids":    fmt.Sprintf(`{"restaurant_id":%q,"menu_item_ids":null}`, restaurantA),
		"trailing garbage":      lookupBody(restaurantA, dishWithRecipe) + "x",
	} {
		if rec := do(h, http.MethodPost, "/internal/recipes/lookup", body, key); rec.Code != http.StatusBadRequest {
			t.Errorf("%s: want 400, got %d", name, rec.Code)
		}
	}
	// An explicit empty list is valid.
	if rec := do(h, http.MethodPost, "/internal/recipes/lookup", fmt.Sprintf(`{"restaurant_id":%q,"menu_item_ids":[]}`, restaurantA), key); rec.Code != http.StatusOK {
		t.Errorf("empty list: want 200, got %d", rec.Code)
	}
}

func TestRecipe_admin_PUT_is_403_and_malformed_id_is_400(t *testing.T) {
	h := newTestRouter()
	admin := map[string]string{"Authorization": "Bearer " + token(t, "admin")}
	if rec := do(h, http.MethodPut, "/api/menu/"+dishWithRecipe+"/recipe", `{"lines":[]}`, admin); rec.Code != http.StatusForbidden {
		t.Errorf("admin PUT: want 403, got %d", rec.Code)
	}
	mgr := map[string]string{"Authorization": "Bearer " + token(t, "manager")}
	if rec := do(h, http.MethodGet, "/api/menu/not-a-uuid/recipe", "", mgr); rec.Code != http.StatusBadRequest {
		t.Errorf("GET malformed id: want 400, got %d", rec.Code)
	}
	if rec := do(h, http.MethodPut, "/api/menu/not-a-uuid/recipe", `{"lines":[]}`, mgr); rec.Code != http.StatusBadRequest {
		t.Errorf("PUT malformed id: want 400, got %d", rec.Code)
	}
}

func seedRecipe(t *testing.T, h http.Handler) {
	t.Helper()
	body := fmt.Sprintf(`{"lines":[{"stock_item_id":%q,"quantity":1.5}]}`, stockFlour)
	rec := do(h, http.MethodPut, "/api/menu/"+dishWithRecipe+"/recipe", body,
		map[string]string{"Authorization": "Bearer " + token(t, "manager")})
	if rec.Code != http.StatusOK {
		t.Fatalf("seed failed: %d %s", rec.Code, rec.Body)
	}
}

func lookupBody(restaurant string, ids ...string) string {
	b, _ := json.Marshal(map[string]any{"restaurant_id": restaurant, "menu_item_ids": ids})
	return string(b)
}

func TestInternal_lookup_without_key_is_401(t *testing.T) {
	h := newTestRouter()
	body := lookupBody(restaurantA, dishWithRecipe)
	if rec := do(h, http.MethodPost, "/internal/recipes/lookup", body, nil); rec.Code != http.StatusUnauthorized {
		t.Errorf("no key: want 401, got %d", rec.Code)
	}
	if rec := do(h, http.MethodPost, "/internal/recipes/lookup", body, map[string]string{"X-Service-Key": "wrong"}); rec.Code != http.StatusUnauthorized {
		t.Errorf("wrong key: want 401, got %d", rec.Code)
	}
	// A JWT must not open the internal route.
	if rec := do(h, http.MethodPost, "/internal/recipes/lookup", body, map[string]string{"Authorization": "Bearer " + token(t, "manager")}); rec.Code != http.StatusUnauthorized {
		t.Errorf("jwt only: want 401, got %d", rec.Code)
	}
}

func TestServiceKey_empty_configured_key_fails_closed(t *testing.T) {
	h := ServiceKey("")(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) }))
	if rec := do(h, http.MethodPost, "/", "", nil); rec.Code != http.StatusUnauthorized {
		t.Fatalf("empty key and empty header must be 401, got %d", rec.Code)
	}
}

func TestInternal_lookup_with_key_returns_recipes_map_omitting_untracked_dishes(t *testing.T) {
	h := newTestRouter()
	seedRecipe(t, h)
	rec := do(h, http.MethodPost, "/internal/recipes/lookup", lookupBody(restaurantA, dishWithRecipe, dishNoRecipe),
		map[string]string{"X-Service-Key": testServiceKey})
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", rec.Code, rec.Body)
	}
	var got struct {
		Recipes map[string][]map[string]any `json:"recipes"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Recipes) != 1 {
		t.Fatalf("want only the tracked dish, got %v", got.Recipes)
	}
	lines := got.Recipes[dishWithRecipe]
	if len(lines) != 1 || lines[0]["stock_item_id"] != stockFlour || lines[0]["quantity"] != 1.5 {
		t.Fatalf("unexpected lines %v", lines)
	}
	if _, ok := lines[0]["stock_item_name"]; ok {
		t.Error("lookup lines must only carry stock_item_id and quantity")
	}
}

func TestInternal_lookup_validation(t *testing.T) {
	h := newTestRouter()
	key := map[string]string{"X-Service-Key": testServiceKey}
	if rec := do(h, http.MethodPost, "/internal/recipes/lookup", lookupBody("", dishWithRecipe), key); rec.Code != http.StatusBadRequest {
		t.Errorf("empty restaurant_id: want 400, got %d", rec.Code)
	}
	if rec := do(h, http.MethodPost, "/internal/recipes/lookup", lookupBody(restaurantA, "not-a-uuid"), key); rec.Code != http.StatusBadRequest {
		t.Errorf("malformed id: want 400, got %d", rec.Code)
	}
	tooMany := make([]string, maxLookupItemIDs+1)
	for i := range tooMany {
		tooMany[i] = dishWithRecipe
	}
	if rec := do(h, http.MethodPost, "/internal/recipes/lookup", lookupBody(restaurantA, tooMany...), key); rec.Code != http.StatusBadRequest {
		t.Errorf("too many ids: want 400, got %d", rec.Code)
	}
	if rec := do(h, http.MethodPost, "/internal/recipes/lookup", `{bad`, key); rec.Code != http.StatusBadRequest {
		t.Errorf("bad json: want 400, got %d", rec.Code)
	}
}

func TestInternal_route_not_under_api(t *testing.T) {
	rec := do(newTestRouter(), http.MethodPost, "/api/recipes/lookup", "{}", map[string]string{"X-Service-Key": testServiceKey})
	if rec.Code != http.StatusNotFound && rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("lookup must not be exposed under /api, got %d", rec.Code)
	}
}
