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
	listmap := make(map[string]string)
	listmap["MY_LIST"] = "one two three"

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
			inputdata: readfile("test/prerendered.yaml"),
			inputvars: listmap,
			want:      readfile("test/rendered.yaml"),
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := Render(c.inputdata, c.inputvars)
			if err != nil {
				fmt.Println("Testing if folder doesnt exist")
			}
			if !reflect.DeepEqual(got, c.want) {
				t.Errorf("got: %#v\nwant: %#v\n", got, c.want)
			}
		})
	}
}
