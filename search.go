package search

import (
	"context"
	"html/template"
	"strconv"
	"strings"

	"github.com/blugelabs/bluge"
	"github.com/blugelabs/bluge/search/highlight"
)

// A pool contains the documents and the search index reader.
type Pool[T any] struct {
	documents []T
	fields    map[string]func(T) string // field name => getter, all given fields are indexed
	reader    *bluge.Reader
}

type Result[T any] struct {
	Document   T
	Highlights map[string]template.HTML
}

// Highlight returns r.Highlights[field]. It is just a convenient helper for templates.
func (r Result[T]) Highlight(field string) template.HTML {
	return r.Highlights[field]
}

func MakePool[T any](documents []T, fields map[string]func(T) string) (*Pool[T], error) {
	var batch = bluge.NewBatch()
	for i, doc := range documents {
		id := strconv.Itoa(i) // slice index becomes document ID
		blugeDoc := bluge.NewDocument(id)
		for name, get := range fields {
			value := get(doc)
			blugeDoc.AddField(bluge.NewTextField(name, value).WithAnalyzer(normalizeAnalyzer).SearchTermPositions().StoreValue())
		}
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

func (pool *Pool[T]) Search(request bluge.SearchRequest) ([]Result[T], error) {
	iterator, err := pool.reader.Search(context.Background(), request)
	if err != nil {
		return nil, err
	}

	highlighter := highlight.NewHTMLHighlighter()
	var results []Result[T]
	for match, err := iterator.Next(); match != nil && err == nil; match, err = iterator.Next() {
		var index int
		var highlights = make(map[string]template.HTML)
		if err := match.VisitStoredFields(func(field string, value []byte) bool {
			if field == "_id" {
				if i, err := strconv.Atoi(string(value)); err == nil {
					index = i
				}
			} else {
				if locations, ok := match.Locations[field]; ok {
					if fragment := highlighter.BestFragment(locations, value); len(fragment) > 0 {
						highlights[field] = template.HTML(fragment)
					}
				}
			}
			return true
		}); err != nil {
			return nil, err
		}

		results = append(results, Result[T]{
			Document:   pool.documents[index],
			Highlights: highlights,
		})
	}

	return results, nil
}

func (pool *Pool[T]) Fuzzy(input string, max int) ([]Result[T], error) {
	input = Normalize(input) // because PrefixQuery etc don't use the DefaultSearchAnalyzer

	query := bluge.NewBooleanQuery()
	for _, word := range strings.Fields(input) {
		wordQuery := bluge.NewBooleanQuery()
		for name := range pool.fields {
			fieldQuery := bluge.NewBooleanQuery()
			fieldQuery.AddShould(bluge.NewFuzzyQuery(word).SetField(name).SetFuzziness(1))
			fieldQuery.AddShould(bluge.NewPrefixQuery(word).SetField(name))
			fieldQuery.AddShould(bluge.NewWildcardQuery("*" + word + "*").SetField(name))
			wordQuery.AddShould(fieldQuery)
		}
		query.AddMust(wordQuery)
	}
	return pool.Search(bluge.NewTopNSearch(max, query))
}

func (pool *Pool[T]) Prefix(input string, max int) ([]Result[T], error) {
	input = Normalize(input) // because PrefixQuery does not use the DefaultSearchAnalyzer

	query := bluge.NewBooleanQuery()
	for _, word := range strings.Fields(input) {
		wordQuery := bluge.NewBooleanQuery()
		for name := range pool.fields {
			wordQuery.AddShould(bluge.NewPrefixQuery(word).SetField(name))
		}
		query.AddMust(wordQuery)
	}
	return pool.Search(bluge.NewTopNSearch(max, query))
}
