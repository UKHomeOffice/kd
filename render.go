package main

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"io/ioutil"
	"math/big"
	"strings"
	"text/template"

	"github.com/Masterminds/sprig"
)

var (
	secretUsed = false
	k8Api      K8Api
)

// Render - the function used for rendering templates (with Sprig support)
func Render(k K8Api, tmpl string, vars map[string]string) (string, bool, error) {
	fm := sprig.TxtFuncMap()
	// Preserve old KD functionality (strings param order vs sprig)
	fm["contains"] = strings.Contains
	fm["hasPrefix"] = strings.HasPrefix
	fm["hasSuffix"] = strings.HasSuffix
	fm["split"] = strings.Split
	fm["secret"] = secret
	// Add file function to map
	fm["file"] = fileRender
	fm["fileWith"] = fileRenderWithData
	// Required for lookup function
	k8Api = k
	fm["k8lookup"] = k8lookup
	secretUsed = false
	defer func() {
		if err := recover(); err != nil {
			logError.Fatal(err)
		}
	}()
	t := template.Must(template.New("template").Funcs(fm).Parse(tmpl))
	if allowMissingVariables {
		t.Option("missingkey=default")
	} else {
		t.Option("missingkey=error")
	}
	var b bytes.Buffer
	if err := t.Execute(&b, vars); err != nil {
		return b.String(), secretUsed, err
	}
	// need to replace blank lines because of bad template formating
	return strings.Replace(b.String(), "\n\n", "\n", -1), secretUsed, nil
}

// secret generate a secret
func secret(stringType string, length int) string {
	var (
		upperAlpha   = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
		lowerAlpha   = "abcdefghijklmnopqrstuvwxyz"
		digits       = "0123456789"
		specials     = "_~=+%^*/()[]{}/!@#$?|"
		mysqlSafe    = "_!#^&*()+{}|:<>?="
		yamlSafe     = "_!#^&*()+<>?="
		allowedChars = []byte{}
	)

	switch stringType {
	case "alphanum":
		allowedChars = []byte(upperAlpha + lowerAlpha + digits)
	case "mysql":
		allowedChars = []byte(upperAlpha + lowerAlpha + digits + mysqlSafe)
	case "yaml":
		allowedChars = []byte(upperAlpha + lowerAlpha + digits + yamlSafe)
	default:
		allowedChars = []byte(upperAlpha + lowerAlpha + digits + specials)
	}

	// Resultant buffer for generated string
	buf := make([]byte, length)

	for i := 0; i < length; i++ {
		// number of chars available
		l := big.NewInt(int64(len(allowedChars)))
		// random index into number of chars
		charI, _ := rand.Int(rand.Reader, l)
		// add buffer char
		buf[i] = allowedChars[charI.Uint64()]
	}
	// Need to assign this to prevent compiler problem
	secretUsed = true
	// lastly return the base64 encoded version
	return base64.StdEncoding.EncodeToString(buf)
}

func fileRenderWithData(key string, extra map[string]interface{}) string {
	data, err := ioutil.ReadFile(key)
	if err != nil {
		panic(err.Error())
	}
	templateData := EnvToMap()
	for key, value := range extra {
		templateData[key] = value.(string)
	}
	render, wasSecret, err := Render(k8Api, string(data), templateData)
	if err != nil {
		panic(err.Error())
	}
	secretUsed = wasSecret
	return render
}

func fileRender(key string) string {
	return fileRenderWithData(key, map[string]interface{}{})
}

// k8lookup find a value from a kubernetes object
func k8lookup(kind, name, path string) string {
	data, err := k8Api.Lookup(kind, name, path)
	if err != nil {
		panic(err.Error())
	}
	return data
}
