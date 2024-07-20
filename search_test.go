package search

import (
	"slices"
	"testing"
)

type document struct {
	content string
}

func TestFuzzy(t *testing.T) {
	pool, _ := MakePool[document]([]document{
		{"quick"},
		{"quick brown fox"},
		{"quäck"},
	}, map[string]func(document) string{
		"content": func(doc document) string { return doc.content },
	})

	tests := []struct {
		input string
		want  []document
	}{
		{"quick", []document{{"quick"}, {"quick brown fox"}, {"quäck"}}},
		{"quack", []document{{"quäck"}, {"quick"}, {"quick brown fox"}}},
		{"fox", []document{{"quick brown fox"}}},
		{"föx", []document{{"quick brown fox"}}},
		{"quick fox", []document{{"quick brown fox"}}},
	}

	for _, test := range tests {
		got, _ := pool.Search(Fuzzy(test.input, 10))
		if !slices.Equal(got, test.want) {
			t.Fatalf("got %v, want %v", got, test.want)
		}
	}
}

func TestPrefix(t *testing.T) {
	pool, _ := MakePool[document]([]document{
		{"foo bar baz"},
		{"föö bär bäz"}, // with umlauts
	}, map[string]func(document) string{
		"content": func(doc document) string { return doc.content },
	})

	tests := []struct {
		input string
		want  []document
	}{
		{"foo", []document{{"foo bar baz"}, {"föö bär bäz"}}},
		{"föö", []document{{"foo bar baz"}, {"föö bär bäz"}}}, // query with umlauts
		{"fo ba", []document{{"foo bar baz"}, {"föö bär bäz"}}},
		{"fö bä", []document{{"foo bar baz"}, {"föö bär bäz"}}}, // query with umlauts
	}

	for i, test := range tests {
		got, _ := pool.Search(Prefix(test.input, 10))
		if !slices.Equal(got, test.want) {
			t.Fatalf("row %d: got %v, want %v", i, got, test.want)
		}
	}
}
