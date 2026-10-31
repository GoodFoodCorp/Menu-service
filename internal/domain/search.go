package domain

import "strings"

// DishMatch is a dish found by a customer's search, with the restaurants it
// is offered in: its own restaurant, or — a global dish — every restaurant
// but those listed in HiddenIn.
type DishMatch struct {
	Item     MenuItem
	HiddenIn []string
}

// accented and plain fold "Crème brûlée" into "creme brulee"; the SQL side
// applies the same translate() so both sides compare alike.
const (
	accented = "àâäáãåéèêëíìîïóòôöõúùûüçñÿ"
	plain    = "aaaaaaeeeeiiiiooooouuuucny"
)

var foldReplacer = func() *strings.Replacer {
	from, to := []rune(accented), []rune(plain)
	pairs := make([]string, 0, 2*len(from))
	for i := range from {
		pairs = append(pairs, string(from[i]), string(to[i]))
	}
	return strings.NewReplacer(pairs...)
}()

// FoldSearch lowercases and strips the accents of a search text.
func FoldSearch(s string) string {
	return foldReplacer.Replace(strings.ToLower(strings.TrimSpace(s)))
}

// FoldSQL is the SQL expression folding a column the way FoldSearch does.
func FoldSQL(column string) string {
	return "translate(lower(" + column + "), '" + accented + "', '" + plain + "')"
}
