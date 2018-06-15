package main

import (
	"fmt"
	"log"
	"os"
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

func TestListDirectory(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  []string
	}{
		{
			name:  "Check yaml files exist",
			input: "test/TestListDirectory/",
			want:  []string{"test/TestListDirectory/1-resource.yaml", "test/TestListDirectory/2-resource.yaml", "test/TestListDirectory/a.yaml", "test/TestListDirectory/b.yaml", "test/TestListDirectory/empty.yaml"},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := ListDirectory(c.input)
			if err != nil {
				fmt.Println("Testing if folder doesnt exist")
			}
			if !reflect.DeepEqual(got, c.want) {
				t.Errorf("got: %#v\nwant: %#v\n", got, c.want)
			}
		})
	}
}

func TestFilesExists(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  bool
	}{
		{
			name:  "Check Dockerfile exists",
			input: "./test/deployment.yaml",
			want:  true,
		},
		{
			name:  "Check fake file doesnt exist",
			input: "./imafakefile",
			want:  false,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := FilesExists(c.input)
			if err != nil {
				log.Fatal(err)
			}
			if !reflect.DeepEqual(got, c.want) {
				t.Errorf("got: %#v\nwant: %#v\n", got, c.want)
			}
		})
	}
}

func TestEnvToMap(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "Check env vars are in map",
			input: "SHELL",
			want:  os.Getenv("SHELL"),
		},
		{
			name:  "Check unset env var is not in map",
			input: "RandomUnsetEnvVar",
			want:  "",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			environmap := EnvToMap()
			got := environmap[c.input]
			if !reflect.DeepEqual(got, c.want) {
				t.Errorf("got: %#v\nwant: %#v\n", got, c.want)
			}
		})
	}
}
