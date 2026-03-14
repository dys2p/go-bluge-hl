package search

import (
	"html/template"

	"github.com/blugelabs/bluge"
)

type Pool[T any] struct {
	Documents []T
	reader    *bluge.Reader
}

func MakePool[T any](documents []T, fields map[string]func(T) string) (*Pool[T], error) {
	var batch = bluge.NewBatch()
	addFields(batch, "", documents, fields)
	reader, err := makeReader(batch)
	if err != nil {
		return nil, err
	}
	return &Pool[T]{
		Documents: documents,
		reader:    reader,
	}, nil
}

func (pool *Pool[T]) Close() error {
	return pool.reader.Close()
}

func (pool *Pool[T]) Search(request bluge.SearchRequest) ([]Result[T], error) {
	var documents []Result[T]
	for r, err := range results(pool.reader, request) {
		if err != nil {
			return nil, err
		}
		documents = append(documents, Result[T]{
			Document:   pool.Documents[r.docIndex],
			Highlights: r.highlights,
		})
	}
	return documents, nil
}

type Pool2[A, B any] struct {
	As     []A
	Bs     []B
	reader *bluge.Reader
}

func MakePool2[A, B any](as []A, aFields map[string]func(A) string, bs []B, bFields map[string]func(B) string) (*Pool2[A, B], error) {
	var batch = bluge.NewBatch()
	addFields(batch, "a", as, aFields)
	addFields(batch, "b", bs, bFields)
	reader, err := makeReader(batch)
	if err != nil {
		return nil, err
	}
	return &Pool2[A, B]{
		As:     as,
		Bs:     bs,
		reader: reader,
	}, nil
}

func (pool *Pool2[A, B]) Close() error {
	return pool.reader.Close()
}

func (pool *Pool2[A, B]) Search(request bluge.SearchRequest) ([]Result[A], []Result[B], error) {
	var as []Result[A]
	var bs []Result[B]
	for r, err := range results(pool.reader, request) {
		if err != nil {
			return nil, nil, err
		}

		switch r.collection {
		case "a":
			as = append(as, Result[A]{
				Document:   pool.As[r.docIndex],
				Highlights: r.highlights,
			})
		case "b":
			bs = append(bs, Result[B]{
				Document:   pool.Bs[r.docIndex],
				Highlights: r.highlights,
			})
		}
	}
	return as, bs, nil
}

type Result[T any] struct {
	Document   T                        `json:"document"`
	Highlights map[string]template.HTML `json:"highlights"` // key: field name, value: full content or fragment
}
