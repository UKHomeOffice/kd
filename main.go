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
	"time"

	"github.com/joho/godotenv"
	"github.com/urfave/cli"
	yaml "gopkg.in/yaml.v2"
)

const DeployDelaySeconds = 3

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
			Name:   "debug-templates",
			Usage:  "debug template output",
			EnvVar: "DEBUG_TEMPLATES,PLUGIN_DEBUG_TEMPLATES",
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
			Name:   "config",
			Usage:  "Env file location",
			EnvVar: "CONFIG_FILE,PLUGIN_CONFIG_FILE",
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

	// Load Environment file overrides into the OS Environment Scope
	if c.IsSet("config") {
		err := godotenv.Load(c.String("config"))
		if err != nil {
			return errors.New("Error loading .env file")
		}
	}

	// Check if all files exist first - fail early on building up a list of files
	var files []string
	for _, fn := range c.StringSlice("file") {
		logDebug.Printf("about to open file:%s\n", fn)
		stat, err := os.Stat(fn)
		if err != nil {
			return err
		}
		switch stat.IsDir() {
		case true:
			fileList, err := ListDirectory(fn)
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
		logDebug.Printf("parsing file:%s\n", fn)
		data, err := ioutil.ReadFile(fn)
		if err != nil {
			return err
		}

		rendered, err := Render(string(data), EnvToMap())
		if err != nil {
			return err
		}

		for _, d := range splitYamlDocs(rendered) {
			r := ObjectResource{FileName: fn, Template: []byte(d)}
			resources = append(resources, &r)
		}
	}

	watch := []*ObjectResource{}
	for _, r := range resources {
		if c.Bool("debug-templates") {
			logInfo.Printf("Template:\n" + string(r.Template[:]))
		}
		if err := yaml.Unmarshal(r.Template, &r); err != nil {
			return err
		}
		if err := deploy(c, r); err != nil {
			return err
		}
		if isWatchableResouce(r) {
			watch = append(watch, r)
		}
	}

	for _, r := range watch {
		if err := watchResource(c, r); err != nil {
			return err
		}
	}

	return nil
}

// EnvToMap - creates a map of all environment variables
func EnvToMap() map[string]string {
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
	logDebug.Printf("about to deploy resource %s/%s (from file:%q)", r.Kind, r.Name, r.FileName)
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
	return nil
}

func isWatchableResouce(r *ObjectResource) bool {
	included := false
	watchable := []string{"Deployment", "StatefulSet", "DaemonSet", "Job"}
	for _, item := range watchable {
	  if item == r.Kind {
	    included = true
	    break
	  }
	}
	return included
}

func watchResource(c *cli.Context, r *ObjectResource) error {
	if c.Bool("debug") {
		logDebug.Printf("sleeping %d seconds before checking %s status for the first time", DeployDelaySeconds, r.Kind)
	}
	time.Sleep(DeployDelaySeconds * time.Second)

	if err := updateResourceStatus(c, r); err != nil {
		return err
	}

	if r.Kind == "StatefulSet" || r.Kind == "DaemonSet" {
		if r.ObjectSpec.UpdateStrategy.Type != "RollingUpdate" {
			if c.Bool("debug") {
				logDebug.Printf("Only %s with type of RollingUpdate will be watched for completion", r.Kind)
			}
			return nil
		}
	}

	ticker := time.NewTicker(c.Duration("check-interval"))
	timeout := time.After(c.Duration("timeout"))

	og := r.DeploymentStatus.ObservedGeneration
	ready := false
	var availableResourceCount int32 = 0
	var unavailableResourceCount int32 = 0

	for {
		select {
		case <-timeout:
			return fmt.Errorf("%s rolling update %q timed out after %s", r.Kind, r.Name, c.Duration("timeout").String())
		case <-ticker.C:
			r.DeploymentStatus = DeploymentStatus{}
			// @TODO should a one-off error (perhaps network issue) cause us to completly fail?
			if err := updateResourceStatus(c, r); err != nil {
				return err
			}
			if c.Bool("debug") {
				logDebug.Printf("fetching %s %q status: %+v", r.Kind, r.Name, r.DeploymentStatus)
			}

			ready = false

			switch r.Kind {
			case "Deployment":
				if (r.DeploymentStatus.UnavailableReplicas == 0 && r.DeploymentStatus.AvailableReplicas == r.DeploymentStatus.Replicas) && 
				r.DeploymentStatus.Replicas == r.DeploymentStatus.UpdatedReplicas {
					ready = true
				}
				availableResourceCount = r.DeploymentStatus.AvailableReplicas
				unavailableResourceCount = r.DeploymentStatus.UnavailableReplicas

			case "StatefulSet":
				if (r.DeploymentStatus.ReadyReplicas == r.ObjectSpec.Replicas) && 
				r.DeploymentStatus.CurrentRevision == r.DeploymentStatus.UpdateRevision {
					ready = true
				}
				availableResourceCount = r.DeploymentStatus.ReadyReplicas
				unavailableResourceCount = r.ObjectSpec.Replicas-r.DeploymentStatus.ReadyReplicas

			case "DaemonSet":
				if (r.DeploymentStatus.DesiredNumberScheduled == r.DeploymentStatus.NumberAvailable) && 
				(r.DeploymentStatus.UpdatedNumberScheduled == r.DeploymentStatus.DesiredNumberScheduled) {
					ready = true
				}
				availableResourceCount = r.DeploymentStatus.NumberAvailable
				unavailableResourceCount = r.DeploymentStatus.DesiredNumberScheduled-r.DeploymentStatus.UpdatedNumberScheduled

			case "Job":
				if r.DeploymentStatus.Succeeded == 1 {
					availableResourceCount = 1
					ready = true	
				}
				unavailableResourceCount = 1
			}

			if ready {
				logInfo.Printf("%s %q is complete. Available objects: %d\n", r.Kind, r.Name, availableResourceCount)
				return nil
			}
			logInfo.Printf("%s %q update in progress. Waiting for %d objects.\n", r.Kind, r.Name, unavailableResourceCount)

			// Fail the deployment in case another deployment has started
			if og != r.DeploymentStatus.ObservedGeneration && c.Bool("fail-superseded") {
				return fmt.Errorf("%s %q update failed. It has been superseded by another update", r.Kind, r.Name)
			}
		}
	}
}

func updateResourceStatus(c *cli.Context, r *ObjectResource) error {
	args := []string{"get", r.Kind + "/" + r.Name, "-o", "yaml"}
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

	flags, err := extraFlags(c)
	if err != nil {
		return nil, err
	}
	args = append(args, flags...)

	return exec.Command(kube, args...), nil
}

func extraFlags(c *cli.Context) ([]string, error) {
	var a []string

	if c.NArg() < 1 {
		return a, nil
	}

	if c.Args()[0] == "--" {
		return c.Args()[1:], nil
	}

	return c.Args(), nil
}

// ListDirectory returns a recursive list of all files under a directory, or an error
func ListDirectory(path string) ([]string, error) {
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
	if found, err := FilesExists(path); err != nil {
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

// FilesExists checks if a file exists already
func FilesExists(path string) (bool, error) {
	stat, err := os.Stat(path)
	if err != nil {
		if err != nil && os.IsNotExist(err) {
			return false, nil
		}

		return false, err
	}

	return !stat.IsDir(), nil
}
