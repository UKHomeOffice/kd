package main

import (
	"bytes"
	"strings"
	"text/template"
)

// Render - renders the templates using the functions in the FuncMap
func Render(tmpl string, vars map[string]string) (string, error) {

	fm := template.FuncMap{
		"split": strings.Split,
	}

	t := template.Must(template.New("template").Funcs(fm).Parse(tmpl))
	t.Option("missingkey=error")
	var b bytes.Buffer
	if err := t.Execute(&b, vars); err != nil {
		return b.String(), err
	}
	return b.String(), nil
}
