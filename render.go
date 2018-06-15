package main

import (
	"bytes"
	"io/ioutil"
	"strings"
	"text/template"

	"github.com/Masterminds/sprig"
)

// Render - the function used for rendering templates (with Sprig support)
func Render(tmpl string, vars map[string]string) (string, error) {
	fm := sprig.TxtFuncMap()
	// Preserve old KD functionality (strings param order vs sprig)
	fm["contains"] = strings.Contains
	fm["hasPrefix"] = strings.HasPrefix
	fm["hasSuffix"] = strings.HasSuffix
	fm["split"] = strings.Split
	// Add file function to map
	fm["file"] = fileRender
	defer func() {
		if err := recover(); err != nil {
			logError.Fatal(err)
		}
	}()
	t := template.Must(template.New("template").Funcs(fm).Parse(tmpl))
	t.Option("missingkey=error")
	var b bytes.Buffer
	if err := t.Execute(&b, vars); err != nil {
		return b.String(), err
	}
	// need to replace blank lines because of bad template formating
	return strings.Replace(b.String(), "\n\n", "\n", -1), nil
}

func fileRender(key string) string {
	data, err := ioutil.ReadFile(key)
	if err != nil {
		panic(err.Error())
	}
	render, err := Render(string(data), EnvToMap())
	if err != nil {
		panic(err.Error())
	}
	return render
}
