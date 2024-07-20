package search

import (
	"context"
	"html/template"
	"strconv"
	"strings"

	"github.com/blugelabs/bluge"
	"github.com/blugelabs/bluge/search/highlight"
	"golang.org/x/exp/maps"
)

// A pool contains the documents and the search index reader.
type Pool[T any] struct {
	documents []T
	fields    map[string]func(T) string // field name => getter, all given fields are indexed
	reader    *bluge.Reader
}

type Result[T any] struct {
	Document T                        `json:"document"`
	HTML     map[string]template.HTML `json:"html"` // both highlighted and non-highlighted fields
}

func MakePool[T any](documents []T, fields map[string]func(T) string) (*Pool[T], error) {
	var fieldNames = maps.Keys(fields)
	var batch = bluge.NewBatch()
	for i, doc := range documents {
		id := strconv.Itoa(i) // slice index becomes document ID
		blugeDoc := bluge.NewDocument(id)
		for name, get := range fields {
			value := get(doc)
			blugeDoc.AddField(bluge.NewTextField(name, value).WithAnalyzer(normalizeAnalyzer).SearchTermPositions().StoreValue())
		}
		blugeDoc.AddField(bluge.NewCompositeFieldIncluding("_all", fieldNames))
		batch.Update(blugeDoc.ID(), blugeDoc)
	}

	config := bluge.InMemoryOnlyConfig()
	config.DefaultSearchAnalyzer.TokenFilters = append(config.DefaultSearchAnalyzer.TokenFilters, normalizeTokenFilter{}) // for search queries, not for index

	index, err := bluge.OpenWriter(config)
	if err != nil {
		return nil, err
	}
	defer index.Close()
	if err := index.Batch(batch); err != nil {
		return nil, err
	}

	reader, err := index.Reader()
	if err != nil {
		return nil, err
	}

	return &Pool[T]{
		documents: documents,
		fields:    fields,
		reader:    reader,
	}, nil
}

func (pool *Pool[T]) Close() error {
	return pool.reader.Close()
}

func (pool *Pool[T]) Search(request bluge.SearchRequest) ([]T, error) {
	iterator, err := pool.reader.Search(context.Background(), request)
	if err != nil {
		return nil, err
	}

	var results []T
	for match, err := iterator.Next(); match != nil && err == nil; match, err = iterator.Next() {
		var index int
		if err := match.VisitStoredFields(func(field string, value []byte) bool {
			if field == "_id" {
				if i, err := strconv.Atoi(string(value)); err == nil {
					index = i
				}
			}
			return true
		}); err != nil {
			return nil, err
		}
		results = append(results, pool.documents[index])
	}
	return results, nil
}

func (pool *Pool[T]) SearchHighlight(request *bluge.TopNSearch) ([]Result[T], error) {
	request = request.IncludeLocations()

	iterator, err := pool.reader.Search(context.Background(), request)
	if err != nil {
		return nil, err
	}

	highlighter := highlight.NewHTMLHighlighter()
	var results []Result[T]
	for match, err := iterator.Next(); match != nil && err == nil; match, err = iterator.Next() {
		var index int
		var html = make(map[string]template.HTML)
		if err := match.VisitStoredFields(func(field string, value []byte) bool {
			switch field {
			case "_id":
				if i, err := strconv.Atoi(string(value)); err == nil {
					index = i
				}
			case "_all":
				// no highlighting
			default:
				if locations, ok := match.Locations[field]; ok {
					if fragment := highlighter.BestFragment(locations, value); len(fragment) > 0 {
						html[field] = template.HTML(fragment)
					}
				}
			}
			return true
		}); err != nil {
			return nil, err
		}

		// add missing (non-highlighted) fields
		for name, get := range pool.fields {
			if _, ok := html[name]; !ok {
				html[name] = template.HTML(get(pool.documents[index]))
			}
		}
		results = append(results, Result[T]{
			Document: pool.documents[index],
			HTML:     html,
		})
	}
	return results, nil
}

func Fuzzy(input string, max int) *bluge.TopNSearch {
	input = Normalize(input) // because PrefixQuery etc don't use the DefaultSearchAnalyzer
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
	input = Normalize(input) // because PrefixQuery doesn't use the DefaultSearchAnalyzer
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
