package domain

import (
	"sort"
	"strings"
)

// Allergens are the 14 substances of annex II of regulation (EU) 1169/2011
// (INCO). In France their presence must be told for every dish sold, distance
// selling included, before the purchase (décret n° 2015-447).
var Allergens = []string{
	"gluten", "crustaces", "oeufs", "poissons", "arachides", "soja", "lait",
	"fruits_a_coque", "celeri", "moutarde", "sesame", "sulfites", "lupin", "mollusques",
}

var knownAllergens = func() map[string]bool {
	m := make(map[string]bool, len(Allergens))
	for _, a := range Allergens {
		m[a] = true
	}
	return m
}()

// CleanAllergens checks the codes and returns them sorted, without duplicates.
func CleanAllergens(in []string) ([]string, error) {
	seen := map[string]bool{}
	out := []string{}
	for _, a := range in {
		a = strings.ToLower(strings.TrimSpace(a))
		if a == "" || seen[a] {
			continue
		}
		if !knownAllergens[a] {
			return nil, NewValidationError("unknown allergen: " + a)
		}
		seen[a] = true
		out = append(out, a)
	}
	sort.Strings(out)
	return out, nil
}
