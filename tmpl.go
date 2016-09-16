package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"path"
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

func justRender(r *ObjectResource, vars map[string]string, debug bool, filename string) error {
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

	var cwdDir, renderDir string
	cwdDir, err = os.Getwd()
	if err != nil {
		logError.Printf("cannot find the current working directory, I've tried here: %v", cwdDir)
		return err
	}
	renderDir = fmt.Sprintf("%v/.kd", cwdDir)
	if _, err := os.Stat(renderDir); err != nil {
		logInfo.Printf("creating render directory at: %v", renderDir)
		if err := os.Mkdir(renderDir, 0755); err != nil {
			logError.Printf("unable to create dir: %v", renderDir)
			return err
		}
	}

	var templateFiles string
	templateFiles = fmt.Sprintf("%v/%v", renderDir, path.Base(filename))
	logInfo.Printf("rendering template %v in file %v", path.Base(filename), templateFiles)
	if err := ioutil.WriteFile(templateFiles, r.Template, 0644); err != nil {
		logError.Printf("can't write rendered templates to file: %v", templateFiles)
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
