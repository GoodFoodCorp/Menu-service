package domain

import (
	"strings"

	"github.com/google/uuid"
)

// MenuItem is a dish on a restaurant's menu. Every item belongs to exactly one
// restaurant (TenantID) — this is the tenant isolation boundary: a manager only
// ever sees or edits items of their own restaurant.
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
}

// MenuItemInput carries the mutable fields for create/update.
type MenuItemInput struct {
	Name        string
	Description string
	PriceCents  int64
	Category    string
	Emoji       string
	Available   bool
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
	return nil
}

// NewMenuItem builds a valid item owned by the given restaurant.
func NewMenuItem(tenantID string, in MenuItemInput, sortOrder int) (*MenuItem, error) {
	if tenantID == "" {
		return nil, NewValidationError("restaurant id is required")
	}
	if err := in.validate(); err != nil {
		return nil, err
	}
	emoji := in.Emoji
	if emoji == "" {
		emoji = "🍽️"
	}
	return &MenuItem{
		ID:          uuid.NewString(),
		TenantID:    tenantID,
		Name:        strings.TrimSpace(in.Name),
		Description: strings.TrimSpace(in.Description),
		PriceCents:  in.PriceCents,
		Category:    strings.TrimSpace(in.Category),
		Emoji:       emoji,
		Available:   in.Available,
		SortOrder:   sortOrder,
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
	return nil
}

// IsOwnedBy reports whether the item belongs to the given restaurant.
func (m *MenuItem) IsOwnedBy(tenantID string) bool { return m.TenantID == tenantID }
