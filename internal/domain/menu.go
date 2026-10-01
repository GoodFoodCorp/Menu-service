package domain

import (
	"strings"

	"github.com/google/uuid"
)

// MenuItem is a dish ("plat") on a restaurant's menu. TenantID is the tenant
// isolation boundary: a manager only ever sees or edits items of their own
// restaurant. An empty TenantID means the item is global — owned by head
// office (admin) and common to every restaurant (root README §6).
type MenuItem struct {
	ID          string
	TenantID    string
	Name        string
	Description string
	PriceCents  int64
	Category    string
	Emoji       string
	Rating      float64
	Available   bool
	SortOrder   int
	// Ingredients is a free-text snapshot, not a live foreign key: on the
	// franchisee side it's typically picked from their own stock-service
	// inventory at authoring time, but stock stays the source of truth for
	// actual quantities — this field is purely compositional/display.
	Ingredients []string
	// HiddenForViewer is transient (not a column on menu_items itself): set
	// only when listing a global item for a specific franchisee, from
	// menu_item_tenant_overrides. A franchisee can hide a default dish from
	// their own restaurant without touching the item everyone else sees.
	HiddenForViewer bool
	// ImageDataURL is an optional photo, stored inline as a data: URL
	// ("data:image/jpeg;base64,...") — see validateImage for the size cap.
	// No object storage in this stack yet, so the DB is the simplest place
	// to keep it; the frontend downsizes/compresses before upload.
	ImageDataURL string
}

// MenuItemInput carries the mutable fields for create/update.
type MenuItemInput struct {
	Name         string
	Description  string
	PriceCents   int64
	Category     string
	Emoji        string
	Available    bool
	Ingredients  []string
	ImageDataURL string
}

func (in MenuItemInput) validate() error {
	if strings.TrimSpace(in.Name) == "" {
		return NewValidationError("item name is required")
	}
	if strings.TrimSpace(in.Category) == "" {
		return NewValidationError("item category is required")
	}
	if in.PriceCents <= 0 {
		return NewValidationError("item price must be greater than zero")
	}
	if err := validateImage(in.ImageDataURL); err != nil {
		return err
	}
	return nil
}

// maxImageDataURLLen caps the stored photo around ~3MB decoded (base64
// inflates size by ~4/3) — generous for a client-side-compressed JPEG/WebP,
// tight enough to keep rows and API responses reasonable.
const maxImageDataURLLen = 4_000_000

func validateImage(dataURL string) error {
	if dataURL == "" {
		return nil
	}
	if !strings.HasPrefix(dataURL, "data:image/") {
		return NewValidationError("image must be a data:image/... URL")
	}
	if len(dataURL) > maxImageDataURLLen {
		return NewValidationError("image is too large (max ~3MB)")
	}
	return nil
}

func cleanStrings(in []string) []string {
	out := make([]string, 0, len(in))
	for _, s := range in {
		s = strings.TrimSpace(s)
		if s != "" {
			out = append(out, s)
		}
	}
	return out
}

// NewMenuItem builds a valid item. tenantID is empty for a global (head
// office) item, or a restaurant id for a franchisee's own item — the caller
// (application layer) decides which is allowed for the current actor.
func NewMenuItem(tenantID string, in MenuItemInput, sortOrder int) (*MenuItem, error) {
	if err := in.validate(); err != nil {
		return nil, err
	}
	emoji := in.Emoji
	if emoji == "" {
		emoji = "🍽️"
	}
	return &MenuItem{
		ID:           uuid.NewString(),
		TenantID:     tenantID,
		Name:         strings.TrimSpace(in.Name),
		Description:  strings.TrimSpace(in.Description),
		PriceCents:   in.PriceCents,
		Category:     strings.TrimSpace(in.Category),
		Emoji:        emoji,
		Available:    in.Available,
		SortOrder:    sortOrder,
		Ingredients:  cleanStrings(in.Ingredients),
		ImageDataURL: in.ImageDataURL,
	}, nil
}

// ApplyUpdate mutates an existing item with validated input.
func (m *MenuItem) ApplyUpdate(in MenuItemInput) error {
	if err := in.validate(); err != nil {
		return err
	}
	m.Name = strings.TrimSpace(in.Name)
	m.Description = strings.TrimSpace(in.Description)
	m.PriceCents = in.PriceCents
	m.Category = strings.TrimSpace(in.Category)
	if in.Emoji != "" {
		m.Emoji = in.Emoji
	}
	m.Available = in.Available
	m.Ingredients = cleanStrings(in.Ingredients)
	m.ImageDataURL = in.ImageDataURL
	return nil
}

// IsOwnedBy reports whether the item is in the given scope — a restaurant id,
// or "" for the global (head office) scope.
func (m *MenuItem) IsOwnedBy(scope string) bool { return m.TenantID == scope }

// IsGlobal reports whether the item is common to every restaurant.
func (m *MenuItem) IsGlobal() bool { return m.TenantID == "" }

// MenuPlan is a named, priced bundle of dishes ("menu" / "formule") — e.g.
// "Menu Burger" = burger + frites + boisson. Same tenant-scoping rules as
// MenuItem: empty TenantID means global (head office).
type MenuPlan struct {
	ID          string
	TenantID    string
	Name        string
	Description string
	PriceCents  int64
	Emoji       string
	Available   bool
	SortOrder   int
	DishIDs     []string
	// HiddenForViewer — see MenuItem.HiddenForViewer.
	HiddenForViewer bool
	// ImageDataURL — see MenuItem.ImageDataURL.
	ImageDataURL string
}

type MenuPlanInput struct {
	Name         string
	Description  string
	PriceCents   int64
	Emoji        string
	Available    bool
	DishIDs      []string
	ImageDataURL string
}

func (in MenuPlanInput) validate() error {
	if strings.TrimSpace(in.Name) == "" {
		return NewValidationError("menu name is required")
	}
	if in.PriceCents <= 0 {
		return NewValidationError("menu price must be greater than zero")
	}
	if len(cleanStrings(in.DishIDs)) == 0 {
		return NewValidationError("a menu needs at least one dish")
	}
	if err := validateImage(in.ImageDataURL); err != nil {
		return err
	}
	return nil
}

func NewMenuPlan(tenantID string, in MenuPlanInput, sortOrder int) (*MenuPlan, error) {
	if err := in.validate(); err != nil {
		return nil, err
	}
	emoji := in.Emoji
	if emoji == "" {
		emoji = "🍽️"
	}
	return &MenuPlan{
		ID:           uuid.NewString(),
		TenantID:     tenantID,
		Name:         strings.TrimSpace(in.Name),
		Description:  strings.TrimSpace(in.Description),
		PriceCents:   in.PriceCents,
		Emoji:        emoji,
		Available:    in.Available,
		SortOrder:    sortOrder,
		DishIDs:      cleanStrings(in.DishIDs),
		ImageDataURL: in.ImageDataURL,
	}, nil
}

func (p *MenuPlan) ApplyUpdate(in MenuPlanInput) error {
	if err := in.validate(); err != nil {
		return err
	}
	p.Name = strings.TrimSpace(in.Name)
	p.Description = strings.TrimSpace(in.Description)
	p.PriceCents = in.PriceCents
	if in.Emoji != "" {
		p.Emoji = in.Emoji
	}
	p.Available = in.Available
	p.DishIDs = cleanStrings(in.DishIDs)
	p.ImageDataURL = in.ImageDataURL
	return nil
}

func (p *MenuPlan) IsOwnedBy(scope string) bool { return p.TenantID == scope }

// IsGlobal reports whether the menu is common to every restaurant.
func (p *MenuPlan) IsGlobal() bool { return p.TenantID == "" }
