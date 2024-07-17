package search

import (
	"strings"
	"unicode"

	"github.com/blugelabs/bluge/analysis"
	"github.com/blugelabs/bluge/analysis/analyzer"
	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

// Normalize is used internally and can be used on input queries which don't have an analyzer, e. g. bluge.PrefixQuery.
func Normalize(s string) string {
	var transformer = transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC) // crashes on parallel use, and also probably stateful, but reusable with Reset
	s, _, _ = transform.String(transformer, s)
	s = strings.ToLower(s)
	return s
}

// for input text, so text highlighting keeps working
var normalizeAnalyzer = func() *analysis.Analyzer {
	var a = analyzer.NewStandardAnalyzer()
	a.TokenFilters = append(a.TokenFilters, normalizeTokenFilter{})
	return a
}()

// for search queries via DefaultSearchAnalyzer
type normalizeTokenFilter struct{}

func (normalizeTokenFilter) Filter(input analysis.TokenStream) analysis.TokenStream {
	var output = make(analysis.TokenStream, len(input))
	for i := range input {
		output[i] = input[i]
		output[i].Term = []byte(Normalize(string(input[i].Term)))
	}
	return output
}
