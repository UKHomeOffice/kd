package main

import (
	"bytes"
	"io/ioutil"
	"os"
	"strings"
	"text/template"
)

func render(r *ObjectResource, vars map[string]string, debug bool) error {
	if debug {
		logDebug.Printf("rendering %q template.", r.FileName)
	}
	tmpl, err := ioutil.ReadFile(r.FileName)
	if err != nil {
		return err
	}
	t := template.Must(template.New(r.FileName).Parse(string(tmpl)))
	t.Option("missingkey=error")
	var b bytes.Buffer
	if err = t.Execute(&b, vars); err != nil {
		return err
	}
	r.Template = b.Bytes()
	if debug {
		logDebug.Printf("template content:\n" + string(r.Template))
	}
	return nil
}

func envToMap() map[string]string {
	m := map[string]string{}
	for _, n := range os.Environ() {
		parts := strings.SplitN(n, "=", 2)
		m[parts[0]] = parts[1]
	}
	return m
}
