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

func TestGetConfigData(t *testing.T) {
	type ConfigFile struct {
		file  string
		scope string
	}

	cases := []struct {
		name               string
		configFiles        []ConfigFile
		templateFile       string
		wantFile           string
		scope              string
		allowMissingValues bool
	}{
		{
			name: "Check simple.yaml file works",
			configFiles: []ConfigFile{
				{file: "./test/TestConfigData/simple.yaml"}},
			templateFile: "./test/TestConfigData/simple.yaml.tmpl",
			wantFile:     "./test/TestConfigData/simple.yaml",
		},
		{
			name: "Check nested-data.yaml file works",
			configFiles: []ConfigFile{
				{file: "./test/TestConfigData/nested-data.yaml"},
			},
			templateFile: "./test/TestConfigData/nested-data.yaml.tmpl",
			wantFile:     "./test/TestConfigData/nested-data.yaml",
		},
		{
			name: "Check data-with-env.yaml file works",
			configFiles: []ConfigFile{
				{file: "./test/TestConfigData/data-with-env.yaml"},
			},
			templateFile: "./test/TestConfigData/data-with-env.yaml.tmpl",
			wantFile:     "./test/TestConfigData/data-with-env.yaml",
		},
		{
			name: "Check chart.yaml with variable.yaml works",
			configFiles: []ConfigFile{
				{file: "./test/TestConfigData/chart.yaml", scope: "Chart"},
				{file: "./test/TestConfigData/chart-values.yaml", scope: "Values"},
			},
			templateFile:       "./test/TestConfigData/chart-deploy.yaml.tmpl",
			wantFile:           "./test/TestConfigData/chart-deploy.yaml",
			allowMissingValues: true,
		},
	}
	os.Setenv("KD_TEST_DATA", "test-data")
	// Probably want to test by rendering templates???
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			confMap := make(map[string]interface{})
			var config interface{}
			scoped := false
			if len(c.configFiles) > 1 {
				scoped = true
			}
			for _, cf := range c.configFiles {
				var err error
				// Get config and merge with environment !scoped
				config, err = GetConfigData(cf.file, !scoped)
				if err != nil {
					log.Fatal(err)
				}
				if scoped {

					// add the config to the right key to scope vars with
					confMap[cf.scope] = config
				}
			}
			if scoped {
				config = confMap
			}
			api := NewK8ApiNoop()
			wantText := readfile(c.wantFile)
			allowMissingVariables = c.allowMissingValues
			rendered, _, err := Render(api, readfile(c.templateFile), config)
			if err != nil {
				t.Errorf("Render error - %s", err)
			}
			if !reflect.DeepEqual(rendered, wantText) {
				t.Errorf("got: %#v\nwant: %#v\n", rendered, wantText)
			}
		})
	}
}
