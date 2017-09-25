package main

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"text/template"
	"time"

	"github.com/urfave/cli"
	"gopkg.in/yaml.v2"
)

var (
	// Version is set at compile time, passing -ldflags "-X main.Version=<build version>"
	Version string

	logInfo  *log.Logger
	logError *log.Logger
	logDebug *log.Logger
)

func init() {
	logInfo = log.New(os.Stdout, "[INFO] ", log.Ldate|log.Ltime|log.Lshortfile)
	logError = log.New(os.Stderr, "[ERROR] ", log.Ldate|log.Ltime|log.Lshortfile)
	logDebug = log.New(os.Stderr, "[DEBUG] ", log.Ldate|log.Ltime|log.Lshortfile)
}

func main() {
	app := cli.NewApp()
	app.Name = "kd"
	app.Author = "Vaidas Jablonskis <jablonskis@gmail.com>"
	app.Version = Version
	app.Usage = "simple kubernetes resources deployment tool"

	app.Flags = []cli.Flag{
		cli.BoolFlag{
			Name:   "debug",
			Usage:  "debug output",
			EnvVar: "DEBUG,PLUGIN_DEBUG",
		},
		cli.BoolFlag{
			Name:   "insecure-skip-tls-verify",
			Usage:  "if true, the server's certificate will not be checked for validity",
			EnvVar: "INSECURE_SKIP_TLS_VERIFY,PLUGIN_INSECURE_SKIP_TLS_VERIFY",
		},
		cli.StringFlag{
			Name:   "kube-server, s",
			Usage:  "kubernetes api server `URL`",
			EnvVar: "KUBE_SERVER,PLUGIN_KUBE_SERVER",
		},
		cli.StringFlag{
			Name:   "kube-token, t",
			Usage:  "kubernetes auth `TOKEN`",
			EnvVar: "KUBE_TOKEN,PLUGIN_KUBE_TOKEN",
		},
		cli.StringFlag{
			Name:   "context, c",
			Usage:  "kube config `CONTEXT`",
			EnvVar: "KUBE_CONTEXT,PLUGIN_CONTEXT",
		},
		cli.StringFlag{
			Name:   "namespace, n",
			Usage:  "kubernetes `NAMESPACE`",
			EnvVar: "KUBE_NAMESPACE,PLUGIN_KUBE_NAMESPACE",
		},
		cli.BoolFlag{
			Name:   "fail-superseded",
			Usage:  "fail deployment if it has been superseded by another deployment. WARNING: there are some bugs in kubernetes.",
			EnvVar: "FAIL_SUPERSEDED,PLUGIN_FAIL_SUPERSEDED",
		},
		cli.StringFlag{
			Name:   "certificate-authority",
			Usage:  "the path to a file containing the CA for kubernetes API `PATH`",
			EnvVar: "KUBE_CERTIFICATE_AUTHORITY,PLUGIN_KUBE_CERTIFICATE_AUHORITY",
		},
		cli.StringFlag{
			Name:   "certificate-authority-data",
			Usage:  "the certificate authority data for the kubernetes API `PATH`",
			EnvVar: "KUBE_CERTIFICATE_AUTHORITY_DATA,PLUGIN_KUBE_CERTIFICATE_AUHORITY_DATA",
		},
		cli.StringFlag{
			Name:  "certificate-authority-file",
			Usage: "the path to file the certificate authority file from certifacte-authority-data option",
			Value: "/tmp/kube-ca.pem",
		},
		cli.StringSliceFlag{
			Name:   "file, f",
			Usage:  "the path to a file or directory containing kubernetes resource/s `PATH`",
			EnvVar: "FILES,PLUGIN_FILES",
		},
		cli.DurationFlag{
			Name:   "timeout, T",
			Usage:  "the amount of time to wait for a successful deployment `TIMEOUT`",
			EnvVar: "TIMEOUT,PLUGIN_TIMEOUT",
			Value:  time.Duration(3) * time.Minute,
		},
		cli.DurationFlag{
			Name:   "check-interval",
			Usage:  "deployment status check interval `INTERVAL`",
			EnvVar: "CHECK_INTERVAL,PLUGIN_CHECK_INTERVAL",
			Value:  time.Duration(1000) * time.Millisecond,
		},
	}

	app.Action = func(cx *cli.Context) error {
		if err := run(cx); err != nil {
			logError.Print(err)
			return cli.NewExitError("", 1)
		}

		return nil
	}
	if err := app.Run(os.Args); err != nil {
		logError.Fatal(err)
	}
}

func run(c *cli.Context) error {
	// Check we have some files to process
	if len(c.StringSlice("file")) == 0 {
		return errors.New("no kubernetes resource files specified")
	}

	// Check if all files exist first - fail early on building up a list of files
	var files []string
	for _, fn := range c.StringSlice("file") {
		stat, err := os.Stat(fn)
		if err != nil {
			return err
		}
		switch stat.IsDir() {
		case true:
			fileList, err := listDirectory(fn)
			if err != nil {
				return err
			}
			files = append(files, fileList...)
		default:
			files = append(files, fn)
		}
	}

	// Iterate the list of files and add rendered templates to resources list - fail early.
	resources := []*ObjectResource{}
	for _, fn := range files {
		data, err := ioutil.ReadFile(fn)
		if err != nil {
			return err
		}

		rendered, err := render(string(data), envToMap())
		if err != nil {
			return err
		}

		for _, d := range splitYamlDocs(rendered) {
			r := ObjectResource{FileName: fn, Template: []byte(d)}
			resources = append(resources, &r)
		}
	}

	for _, r := range resources {
		if err := yaml.Unmarshal(r.Template, &r); err != nil {
			return err
		}
		if err := deploy(c, r); err != nil {
			return err
		}
	}
	return nil
}

func render(tmpl string, vars map[string]string) (string, error) {
	t := template.Must(template.New("template").Parse(tmpl))
	t.Option("missingkey=error")
	var b bytes.Buffer
	if err := t.Execute(&b, vars); err != nil {
		return b.String(), err
	}
	return b.String(), nil
}

func envToMap() map[string]string {
	m := map[string]string{}
	for _, n := range os.Environ() {
		parts := strings.SplitN(n, "=", 2)
		m[parts[0]] = parts[1]
	}
	return m
}

// splitYamlDocs splits a yaml string into separate yaml documents.
func splitYamlDocs(data string) []string {
	r := regexp.MustCompile(`(?m)^---\n`)
	s := r.Split(data, -1)
	for i, item := range s {
		if item == "" {
			s = append(s[:i], s[i+1:]...)
		}
	}
	return s
}

func deploy(c *cli.Context, r *ObjectResource) error {
	args := []string{"apply", "-f", "-"}
	cmd, err := newKubeCmd(c, args)
	if err != nil {
		return err
	}

	if c.Bool("debug") {
		logDebug.Printf("kubectl arguments: %q", strings.Join(cmd.Args, " "))
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}

	var outbuf, errbuf bytes.Buffer
	cmd.Stdout = &outbuf
	cmd.Stderr = &errbuf

	if _, err := stdin.Write(r.Template); err != nil {
		return err
	}
	if err := stdin.Close(); err != nil {
		return err
	}
	logInfo.Printf("deploying %s/%s", strings.ToLower(r.Kind), r.Name)
	if err = cmd.Run(); err != nil {
		if errbuf.Len() > 0 {
			return fmt.Errorf(errbuf.String())
		}
		return err
	}
	logInfo.Print(outbuf.String())
	if r.Kind != "Deployment" {
		return nil
	}

	if c.Bool("debug") {
		logDebug.Printf("sleeping 3 seconds before checking deployment status for the first time")
	}
	time.Sleep(3 * time.Second)

	if err := updateDeploymentStatus(c, r); err != nil {
		return err
	}

	ticker := time.NewTicker(c.Duration("check-interval"))
	timeout := time.After(c.Duration("timeout"))
	og := r.DeploymentStatus.ObservedGeneration

	for {
		select {
		case <-timeout:
			return fmt.Errorf("deployment %q timed out after %s", r.Name, c.Duration("timeout").String())
		case <-ticker.C:
			r.DeploymentStatus = DeploymentStatus{}
			// @TODO should a one-off error (perhaps network issue) cause us to completly fail?
			if err := updateDeploymentStatus(c, r); err != nil {
				return err
			}
			if c.Bool("debug") {
				logDebug.Printf("fetching deployment status: %+v", r.DeploymentStatus)
			}

			if (r.DeploymentStatus.UnavailableReplicas == 0 && r.DeploymentStatus.AvailableReplicas == r.DeploymentStatus.Replicas) &&
				r.DeploymentStatus.Replicas == r.DeploymentStatus.UpdatedReplicas {
				logInfo.Printf("deployment %q is complete. Available replicas: %d\n",
					r.Name, r.DeploymentStatus.AvailableReplicas)
				return nil
			}
			logInfo.Printf("deployment %q in progress. Unavailable replicas: %d.\n",
				r.Name, r.DeploymentStatus.UnavailableReplicas)

			// Fail the deployment in case another deployment has started
			if og != r.DeploymentStatus.ObservedGeneration && c.Bool("fail-superseded") {
				return fmt.Errorf("deployment failed. It has been superseded by another deployment")
			}
		}
	}
}

func updateDeploymentStatus(c *cli.Context, r *ObjectResource) error {
	args := []string{"get", "deployment/" + r.Name, "-o", "yaml"}
	cmd, err := newKubeCmd(c, args)
	if err != nil {
		return err
	}
	cmd.Stderr = os.Stderr
	stdout, _ := cmd.StdoutPipe()
	if err := cmd.Start(); err != nil {
		return err
	}
	data, _ := ioutil.ReadAll(stdout)
	if err := yaml.Unmarshal(data, r); err != nil {
		return err
	}
	if err := cmd.Wait(); err != nil {
		return err
	}
	return nil
}

func newKubeCmd(c *cli.Context, args []string) (*exec.Cmd, error) {
	kube := "kubectl"
	if c.IsSet("namespace") {
		args = append([]string{"--namespace=" + c.String("namespace")}, args...)
	}
	if c.IsSet("context") {
		args = append([]string{"--context=" + c.String("context")}, args...)
	}
	if c.IsSet("kube-token") {
		args = append([]string{"--token=" + c.String("kube-token")}, args...)
	}
	if c.IsSet("certificate-authority-data") {
		if err := createCertificateAuthority(c.String("certificate-authority-file"), c.String("certificate-authority-data")); err != nil {
			return nil, err
		}
		args = append([]string{"--certificate-authority=" + c.String("certificate-authority-file")}, args...)
	}
	if c.IsSet("certificate-authority") {
		args = append([]string{"--certificate-authority=" + c.String("certificate-authority")}, args...)
	}
	if c.IsSet("insecure-skip-tls-verify") {
		args = append([]string{"--insecure-skip-tls-verify"}, args...)
	}
	if c.IsSet("kube-server") {
		args = append([]string{"--server=" + c.String("kube-server")}, args...)
	}

	return exec.Command(kube, args...), nil
}

// listDirectory returns a recursive list of all files under a directory, or an error
func listDirectory(path string) ([]string, error) {
	var list []string
	err := filepath.Walk(path, func(path string, info os.FileInfo, err error) error {
		if !info.IsDir() {
			// We only support yaml at the moment, so we might well filter on it
			switch filepath.Ext(path) {
			case ".yaml":
				fallthrough
			case ".yml":
				list = append(list, path)
			}
		}
		return nil
	})

	return list, err
}

// createCertificateAuthority creates if required a certificate-authority file
func createCertificateAuthority(path, content string) error {
	// This hardcoded certificate authority
	if found, err := filesExists(path); err != nil {
		return err
	} else if found {
		return nil
	}

	// Write the file to disk
	if err := ioutil.WriteFile(path, []byte(content), 0444); err != nil {
		return err
	}

	return nil
}

// fileExists checks if a file exists already
func filesExists(path string) (bool, error) {
	stat, err := os.Stat(path)
	if err != nil {
		if err != nil && os.IsNotExist(err) {
			return false, nil
		}

		return false, err
	}

	return !stat.IsDir(), nil
}
