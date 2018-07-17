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
)

// Render - the function used for rendering templates (with Sprig support)
func Render(tmpl string, vars map[string]string) (string, bool, error) {
	fm := sprig.TxtFuncMap()
	// Preserve old KD functionality (strings param order vs sprig)
	fm["contains"] = strings.Contains
	fm["hasPrefix"] = strings.HasPrefix
	fm["hasSuffix"] = strings.HasSuffix
	fm["split"] = strings.Split
	fm["secret"] = secret
	// Add file function to map
	fm["file"] = fileRender
	secretUsed = false
	defer func() {
		if err := recover(); err != nil {
			logError.Fatal(err)
		}
	}()
	t := template.Must(template.New("template").Funcs(fm).Parse(tmpl))
	t.Option("missingkey=error")
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
		upper_alpha  = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
		lower_alpha  = "abcdefghijklmnopqrstuvwxyz"
		digits       = "0123456789"
		specials     = "_~=+%^*/()[]{}/!@#$?|"
		mysqlSafe    = "_!#^&*()+{}|:<>?="
		yamlSafe     = "_!#^&*()+<>?="
		allowedChars = []byte{}
	)

	switch stringType {
	case "alphanum":
		allowedChars = []byte(upper_alpha + lower_alpha + digits)
	case "mysql":
		allowedChars = []byte(upper_alpha + lower_alpha + digits + mysqlSafe)
	case "yaml":
		allowedChars = []byte(upper_alpha + lower_alpha + digits + yamlSafe)
	default:
		allowedChars = []byte(upper_alpha + lower_alpha + digits + specials)
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

func fileRender(key string) string {
	data, err := ioutil.ReadFile(key)
	if err != nil {
		panic(err.Error())
	}
	render, wasSecret, err := Render(string(data), EnvToMap())
	if err != nil {
		panic(err.Error())
	}
	secretUsed = wasSecret
	return render
}
