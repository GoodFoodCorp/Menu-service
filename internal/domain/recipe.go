package domain

import (
	"fmt"
	"math"

	"github.com/google/uuid"
)

const (
	// MaxRecipeQuantity is the largest value NUMERIC(12,3) can store.
	MaxRecipeQuantity = 999_999_999.999
	// MaxRecipeLines bounds the number of ingredients in one recipe.
	MaxRecipeLines = 50
)

// NormalizeUUID returns the lowercase canonical form of a UUID, or a
// validation error naming the field when s is not a valid UUID.
func NormalizeUUID(field, s string) (string, error) {
	id, err := uuid.Parse(s)
	if err != nil {
		return "", NewValidationError(field + " must be a valid UUID")
	}
	return id.String(), nil
}

// RecipeLine is one ingredient of a dish recipe: a stock item of the
// restaurant (referenced by id, with its name and unit denormalised for
// display) and the quantity consumed per dish. Quantity has 3 decimals,
// matching NUMERIC(12,3) in storage.
type RecipeLine struct {
	StockItemID   string
	StockItemName string
	Unit          string
	Quantity      float64
}

// RoundQuantity rounds a quantity to the 3 decimals the storage keeps.
func RoundQuantity(q float64) float64 {
	return math.Round(q*1000) / 1000
}

// ValidateRecipe checks that every line references a stock item, has a
// strictly positive quantity within NUMERIC(12,3), that stock item ids are
// valid UUIDs, and that no stock item appears twice. Ids are normalised to
// lowercase canonical form IN PLACE before the duplicate check. An empty
// recipe is valid (it means "no recipe").
func ValidateRecipe(lines []RecipeLine) error {
	if len(lines) > MaxRecipeLines {
		return NewValidationError(fmt.Sprintf("a recipe has at most %d lines", MaxRecipeLines))
	}
	seen := make(map[string]struct{}, len(lines))
	for i := range lines {
		l := &lines[i]
		if l.StockItemID == "" {
			return NewValidationError(fmt.Sprintf("line %d: stock_item_id is required", i+1))
		}
		id, err := NormalizeUUID("stock_item_id", l.StockItemID)
		if err != nil {
			return NewValidationError(fmt.Sprintf("line %d: stock_item_id must be a valid UUID", i+1))
		}
		l.StockItemID = id
		if math.IsNaN(l.Quantity) || math.IsInf(l.Quantity, 0) || l.Quantity <= 0 {
			return NewValidationError(fmt.Sprintf("line %d: quantity must be greater than 0", i+1))
		}
		if l.Quantity > MaxRecipeQuantity {
			return NewValidationError(fmt.Sprintf("line %d: quantity must not exceed %v", i+1, MaxRecipeQuantity))
		}
		if _, dup := seen[l.StockItemID]; dup {
			return NewValidationError(fmt.Sprintf("line %d: stock item %s appears more than once", i+1, l.StockItemID))
		}
		seen[l.StockItemID] = struct{}{}
	}
	return nil
}
