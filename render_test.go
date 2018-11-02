package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"reflect"
	"testing"
)

var emptymap map[string]string

func readfile(filepath string) string {

	dat, err := ioutil.ReadFile(filepath)
	if err != nil {
		log.Fatal(err)
	}
	return string(dat)
}

func TestRender(t *testing.T) {
	testData := make(map[string]string)
	testData["MY_LIST"] = "one,two,three"
	testData["FILE_PATH"] = "test/complex-file.pem"
	testData["TEMPLATED_FILE_PATH"] = "test/file-with-calculations.yaml.template"

	cases := []struct {
		name      string
		inputdata string
		inputvars map[string]string
		want      string
	}{
		{
			name:      "Check plain file is rendered",
			inputdata: readfile("test/deployment.yaml"),
			inputvars: emptymap,
			want:      readfile("test/deployment.yaml"),
		},
		{
			name:      "Check list variables are rendered",
			inputdata: readfile("test/list-prerendered.yaml"),
			inputvars: testData,
			want:      readfile("test/list-rendered.yaml"),
		},
		{
			name:      "Check file function is rendered",
			inputdata: readfile("test/file-prerendered.yaml"),
			inputvars: testData,
			want:      readfile("test/file-rendered.yaml"),
		},
		{
			name:      "Check fileWith function is rendered",
			inputdata: readfile("test/fileWith-prerendered.yaml"),
			inputvars: testData,
			want:      readfile("test/fileWith-rendered.yaml"),
		},
		{
			name:      "Check contains function works as expected",
			inputdata: readfile("test/contains-prerendered.yaml"),
			inputvars: testData,
			want:      readfile("test/contains-rendered.yaml"),
		},
		{
			name:      "Check hasPrefix function works as expected",
			inputdata: readfile("test/hasPrefix-prerendered.yaml"),
			inputvars: testData,
			want:      readfile("test/hasPrefix-rendered.yaml"),
		},
		{
			name:      "Check hasSuffix function works as expected",
			inputdata: readfile("test/hasSuffix-prerendered.yaml"),
			inputvars: testData,
			want:      readfile("test/hasSuffix-rendered.yaml"),
		},
	}

	api := NewK8ApiNoop()
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, _, err := Render(api, c.inputdata, c.inputvars)
			if err != nil {
				fmt.Println("Testing if folder doesnt exist")
			}
			if !reflect.DeepEqual(got, c.want) {
				t.Errorf("got: %#v\nwant: %#v\n", got, c.want)
			}
		})
	}

	// Test of secret functions:
	t.Run("Check secret is parsed and detected", func(t *testing.T) {
		c := readfile("test/secret.yaml")
		_, isSecret, err := Render(api, c, testData)
		if err != nil {
			fmt.Printf("unexpected problem rendering:%v\n", err)
		}
		if !isSecret {
			t.Errorf("expected secret to be detected from: \n%#v", c)
		}
	})
}
