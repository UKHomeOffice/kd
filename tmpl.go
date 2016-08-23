package main

import (
	"io"
	"os"
	"strings"
	"text/template"
)

func render(r *ObjectResource, vars map[string]string, w io.Writer) error {
	t := template.Must(template.New(r.FileName).Parse(string(r.Template)))
	err := t.Execute(w, vars)
	if err != nil {
		return err
	}
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
