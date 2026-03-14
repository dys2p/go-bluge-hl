package search

import (
	"context"
	"html/template"
	"iter"
	"strconv"
	"strings"

	"github.com/blugelabs/bluge"
	"github.com/blugelabs/bluge/index"
	"github.com/blugelabs/bluge/search/highlight"
	"golang.org/x/exp/maps"
)

type result struct {
	collection string
	docIndex   int
	highlights map[string]template.HTML
}

func addFields[T any](batch *index.Batch, collection string, documents []T, fields map[string]func(T) string) {
	fieldNames := maps.Keys(fields)
	for i, doc := range documents {
		id := collection + "/" + strconv.Itoa(i) // document id = collection id + document index
		blugeDoc := bluge.NewDocument(id)
		for name, get := range fields {
			value := get(doc)
			blugeDoc.AddField(bluge.NewTextField(name, value).WithAnalyzer(normalizeAnalyzer).SearchTermPositions().StoreValue())
		}
		blugeDoc.AddField(bluge.NewCompositeFieldIncluding("_all", fieldNames))
		batch.Update(blugeDoc.ID(), blugeDoc)
	}
}

func makeReader(batch *index.Batch) (*bluge.Reader, error) {
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
	return index.Reader()
}

func results(reader *bluge.Reader, request bluge.SearchRequest) iter.Seq2[result, error] {
	return func(yield func(result, error) bool) {
		iterator, err := reader.Search(context.Background(), request)
		if err != nil {
			yield(result{}, err)
		}
		highlighter := highlight.NewHTMLHighlighter()
		for match, err := iterator.Next(); match != nil && err == nil; match, err = iterator.Next() {
			var res result
			err := match.VisitStoredFields(func(field string, value []byte) bool {
				switch field {
				case "_id":
					collection, indexStr, _ := strings.Cut(string(value), "/")
					res.collection = collection
					res.docIndex, _ = strconv.Atoi(indexStr)
				case "_all":
					// no highlighting
				default:
					if locations, ok := match.Locations[field]; ok {
						if fragment := highlighter.BestFragment(locations, value); len(fragment) > 0 {
							if res.highlights == nil {
								res.highlights = make(map[string]template.HTML)
							}
							res.highlights[field] = template.HTML(fragment)
						}
					}
				}
				return true
			})
			yield(res, err)
		}
	}
}
