package main

import (
	"bytes"
	"io/ioutil"
	"os"
	"strings"
	"text/template"
)

func render(r *ObjectResource, vars map[string]string) error {
	tmpl, err := ioutil.ReadFile(r.FileName)
	if err != nil {
		return err
	}
	var b bytes.Buffer
	t := template.Must(template.New(r.FileName).Parse(string(tmpl)))
	if err = t.Execute(&b, vars); err != nil {
		return err
	}
	r.Template = b.Bytes()
	return nil
}

func envToMap() map[string]string {
	m := map[string]string{}
	for _, n := range os.Environ() {
		parts := strings.Split(n, "=")
		m[parts[0]] = parts[1]
	}
	return m
}
