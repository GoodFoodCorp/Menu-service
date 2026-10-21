package domain

import (
	"fmt"
	"math"
)

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
// strictly positive quantity, and that no stock item appears twice. An empty
// recipe is valid (it means "no recipe").
func ValidateRecipe(lines []RecipeLine) error {
	seen := make(map[string]struct{}, len(lines))
	for i, l := range lines {
		if l.StockItemID == "" {
			return NewValidationError(fmt.Sprintf("line %d: stock_item_id is required", i+1))
		}
		if math.IsNaN(l.Quantity) || math.IsInf(l.Quantity, 0) || l.Quantity <= 0 {
			return NewValidationError(fmt.Sprintf("line %d: quantity must be greater than 0", i+1))
		}
		if _, dup := seen[l.StockItemID]; dup {
			return NewValidationError(fmt.Sprintf("line %d: stock item %s appears more than once", i+1, l.StockItemID))
		}
		seen[l.StockItemID] = struct{}{}
	}
	return nil
}
