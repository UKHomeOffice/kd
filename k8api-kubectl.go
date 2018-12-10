package main

import (
	"fmt"
	"io/ioutil"
	"strings"

	"github.com/urfave/cli"
)

// K8ApiKubectl is a kubectl implimentation of K8Api interface
type K8ApiKubectl struct {
	K8Api
	Cx *cli.Context
}

// NewK8ApiKubectl creates a concrete class bound to use kubectl
func NewK8ApiKubectl(c *cli.Context) K8Api {
	api := &K8ApiKubectl{
		Cx: c,
	}
	return api
}

// Lookup will get data from a specified kubernetes object
func (a K8ApiKubectl) Lookup(kind, name, path string) (string, error) {
	args := []string{"get", kind + "/" + name, "-o", "custom-columns=:" + path, "--no-headers"}

	cmd, err := newKubeCmd(a.Cx, args, false)
	if err != nil {
		return "", err
	}
	stderr, _ := cmd.StderrPipe()
	stdout, _ := cmd.StdoutPipe()
	if err := cmd.Start(); err != nil {
		logDebug.Printf("error starting kubectl: %s", err)
		return "", err
	}
	data, _ := ioutil.ReadAll(stdout)
	if err := cmd.Wait(); err != nil {
		logDebug.Printf("error with kubectl: %s", err)
		errData, _ := ioutil.ReadAll(stderr)
		if strings.Contains("NotFound", string(errData[:])) {
			return "", fmt.Errorf("Error object %s/%s not found", kind, name)
		}
		return "", err
	}
	return strings.TrimSpace(string(data[:])), nil
}
