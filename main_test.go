package main

import (
	"reflect"
	"testing"
)

func TestSplitYamlDocs(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  []string
	}{
		{
			name:  "single yaml document",
			input: "---\nfoo: bar\n",
			want:  []string{"foo: bar\n"},
		},
		{
			name:  "two yaml documents",
			input: "---\nfoo: bar\n---\nanother: doc\n",
			want:  []string{"foo: bar\n", "another: doc\n"},
		},
		{
			name:  "two yaml documents no doc separator",
			input: "foo: bar\n---\nanother: doc\n",
			want:  []string{"foo: bar\n", "another: doc\n"},
		},
		{
			name:  "separator in the middle of the doc",
			input: "foo: '---bar'\n",
			want:  []string{"foo: '---bar'\n"},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := splitYamlDocs(c.input)
			if !reflect.DeepEqual(got, c.want) {
				t.Errorf("got: %#v\nwant: %#v\n", got, c.want)
			}
		})
	}
}
