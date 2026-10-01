package domain

import (
	"strings"

	"github.com/google/uuid"
)

// Category is a dish category (Burgers, Pizzas, Boissons…) — a network-wide
// taxonomy managed by head office, shared by every franchisee's dish forms.
type Category struct {
	ID        string
	Name      string
	SortOrder int
}

func NewCategory(name string, sortOrder int) (*Category, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, NewValidationError("category name is required")
	}
	return &Category{ID: uuid.NewString(), Name: name, SortOrder: sortOrder}, nil
}
