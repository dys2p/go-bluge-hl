package search

import (
	"strings"

	"github.com/blugelabs/bluge"
)

func Fuzzy(input string, max int) *bluge.TopNSearch {
	input = string(Normalize([]byte(input)))
	words := strings.Fields(input)
	if len(words) > 5 {
		words = words[:5]
	}

	query := bluge.NewBooleanQuery()
	for _, word := range words {
		wordQuery := bluge.NewBooleanQuery()
		wordQuery.AddShould(bluge.NewFuzzyQuery(word).SetField("_all").SetFuzziness(1))
		wordQuery.AddShould(bluge.NewPrefixQuery(word).SetField("_all"))
		wordQuery.AddShould(bluge.NewWildcardQuery("*" + word + "*").SetField("_all"))
		query.AddMust(wordQuery)
	}
	return bluge.NewTopNSearch(max, query)
}

func Prefix(input string, max int) *bluge.TopNSearch {
	input = string(Normalize([]byte(input)))
	words := strings.Fields(input)
	if len(words) > 5 {
		words = words[:5]
	}

	query := bluge.NewBooleanQuery()
	for _, word := range words {
		query.AddMust(bluge.NewPrefixQuery(word).SetField("_all"))
	}
	return bluge.NewTopNSearch(max, query)
}
