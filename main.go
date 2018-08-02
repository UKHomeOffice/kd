package main

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/cavaliercoder/grab"
	"github.com/joho/godotenv"
	"github.com/urfave/cli"
	yaml "gopkg.in/yaml.v2"
)

const (
	// DeployDelaySeconds - delay between deployments
	DeployDelaySeconds = 3
	// MaxHealthcheckRetries - Amount of times to retry checking of resource after deployment
	MaxHealthcheckRetries = 3
	// HealthCheckSleepDuration - the amount of time to sleep (seconds) between healthcehck retries
	HealthCheckSleepDuration = time.Duration(int64(2)) * time.Second
	// FlagCreateOnlyResources is the flag syntax to specify create only for only
	// the specified resource
	FlagCreateOnlyResources = "create-only-resource"
	// FlagCreateOnly is the flag syntax to specify create only for all resources
	FlagCreateOnly = "create-only"
	// FlagCa specifies the synatx for specifying a CA to trust
	FlagCa = "certificate-authority"
	// FlagCaData is the flag to specify that a PEM encoded CA is being specified
	FlagCaData = "certificate-authority-data"
	// FlagCaFile is the sytax to specify a CA file when FlagCa specifies a URL or
	// when FlagCaData is set
	FlagCaFile = "certificate-authority-file"
	// FlagReplace allows the resources to be re-created rather than patched
	FlagReplace = "replace"
)

var (
	// Version is set at compile time, passing -ldflags "-X main.Version=<build version>"
	Version string

	logInfo    *log.Logger
	logError   *log.Logger
	logDebug   *log.Logger
	logDebugIf *log.Logger

	// dryRun Defaults to false
	dryRun bool

	// Files to delete on exit
	tmpDir string = ""

	// caFile
	caFile string = ""
)

func init() {
	logInfo = log.New(os.Stdout, "[INFO] ", log.Ldate|log.Ltime|log.Lshortfile)
	logError = log.New(os.Stderr, "[ERROR] ", log.Ldate|log.Ltime|log.Lshortfile)
	logDebugIf = log.New(os.Stderr, "[DEBUG] ", log.Ldate|log.Ltime|log.Lshortfile)
	logDebug = log.New(ioutil.Discard, "", log.Lshortfile)
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
			Name:        "dryrun",
			Usage:       "if true, kd will exit prior to deployment",
			EnvVar:      "DRY_RUN",
			Destination: &dryRun,
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
			Name:   "kube-username, u",
			Usage:  "kubernetes auth `USERNAME`",
			EnvVar: "KUBE_USERNAME,PLUGIN_KUBE_USERNAME",
		},
		cli.StringFlag{
			Name:   "kube-password, p",
			Usage:  "kubernetes auth `PASSWORD`",
			EnvVar: "KUBE_PASSWORD,PLUGIN_KUBE_PASSWORD",
		},
		cli.StringFlag{
			Name:   "config",
			Usage:  "Env file location",
			EnvVar: "CONFIG_FILE,PLUGIN_CONFIG_FILE",
		},
		cli.BoolFlag{
			Name:   FlagCreateOnly,
			Usage:  "only create resources (do not update, skip if exists).",
			EnvVar: "CREATE_ONLY,PLUGIN_CREATE_ONLY",
		},
		cli.StringSliceFlag{
			Name:   FlagCreateOnlyResources,
			Usage:  "only create specified resources e.g. 'kind/name' (do not update, skip if exists).",
			EnvVar: "CREATE_ONLY_RESOURCES,PLUGIN_CREATE_ONLY_RESOURCES",
			Value:  nil,
		},
		cli.BoolFlag{
			Name:   FlagReplace,
			Usage:  "use replace instead of apply for updating objects",
			EnvVar: "KUBE_REPLACE,PLUGIN_KUBE_REPLACE",
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
			Name:   FlagCa,
			Usage:  "the path (or URL) to a file containing the CA for kubernetes API `PATH`",
			EnvVar: "KUBE_CERTIFICATE_AUTHORITY,PLUGIN_KUBE_CERTIFICATE_AUTHORITY",
		},
		cli.StringFlag{
			Name:   FlagCaData,
			Usage:  "the certificate authority data for the kubernetes API `PATH`",
			EnvVar: "KUBE_CERTIFICATE_AUTHORITY_DATA,PLUGIN_KUBE_CERTIFICATE_AUTHORITY_DATA",
		},
		cli.StringFlag{
			Name:   FlagCaFile,
			Usage:  "the path to save certificate authority data to when data or a URL is specified",
			Value:  "/tmp/kube-ca.pem",
			EnvVar: "KUBE_CERTIFICATE_AUTHORITY_FILE,PLUGIN_KUBE_CERTIFICATE_AUTHORITY_FILE",
		},
		cli.StringSliceFlag{
			Name:   "file, f",
			Usage:  "the path to a file or directory containing kubernetes resources `PATH`",
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
	app.Commands = []cli.Command{
		{
			Action:          runKubectl,
			Name:            "run",
			Usage:           "run [kubectl args] - runs kubectl supporting kd flags / environment options",
			Description:     "runs kubectl whist supporting the kd global flags",
			UsageText:       "run [kubectl args] - will run kubectl with all the parameters supplied",
			SkipFlagParsing: true,
			OnUsageError:    nil,
		},
	}

	app.Action = func(cx *cli.Context) error {
		if err := run(cx); err != nil {
			logError.Print(err)
			return cli.NewExitError("", 1)
		}

		return nil
	}
	defer cleanup()
	if err := app.Run(os.Args); err != nil {
		logError.Fatal(err)
	}
}

// Delete any temparay files
func cleanup() {
	if len(tmpDir) > 0 {
		logDebug.Printf("cleaning up %s", tmpDir)
		os.RemoveAll(tmpDir)
	}
}

func runKubectl(c *cli.Context) error {
	if c.Parent().Bool("debug") {
		logDebug = logDebugIf
	}
	if c.Parent().IsSet(FlagCreateOnlyResources) {
		if len(c.Parent().StringSlice(FlagCreateOnlyResources)) > 1 {
			return fmt.Errorf("can only specify a single resource when using run")
		}
		resString := c.Parent().StringSlice(FlagCreateOnlyResources)[0]
		resParts := strings.Split(resString, "/")
		if len(resParts) != 2 {
			return fmt.Errorf(
				"invalid resource type %s, expecting kind/name", resString)
		}
		name := resParts[1]
		kind := resParts[0]
		exists, err := checkResourceExist(c.Parent(), &ObjectResource{
			Kind: kind,
			ObjectMeta: ObjectMeta{
				Name: name,
			},
		})
		if err != nil {
			return fmt.Errorf(
				"problem checking if resource %s/%s exists", name, kind)
		}
		if exists {
			log.Printf(
				"resource marked as 'create only', skipping app for %s", resString)
			return nil
		}
	}

	// Allow the lib to render args and then create array
	cmd, err := newKubeCmdSub(c.Parent(), c.Args(), true)
	if err != nil {
		return err
	}
	logDebug.Printf("About to run %s", cmd.Args)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err = cmd.Run(); err != nil {
		return fmt.Errorf(
			"error running '%s %s' (use debug to see sensitive params):%s",
			cmd.Args[0],
			strings.Join(c.Args(), " "),
			err)
	}
	return nil
}

func run(c *cli.Context) error {
	if c.Bool("debug") {
		logDebug = logDebugIf
	}
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
		for _, d := range splitYamlDocs(string(data)) {
			rendered, genSecret, err := Render(string(d), EnvToMap())
			if err != nil {
				return err
			}
			r := &ObjectResource{
				FileName:   fn,
				Template:   []byte(rendered),
				CreateOnly: genSecret,
			}
			resources = append(resources, r)
		}
	}
	for _, r := range resources {
		if c.Bool("debug-templates") {
			logInfo.Printf("Template:\n" + string(r.Template[:]))
		}
		if err := yaml.Unmarshal(r.Template, &r); err != nil {
			return err
		}
		// Add any flag specific settings for resources
		updateResFromFlags(c, r)

		// Only perform deploy if dry-run is not set to true
		if !dryRun {
			if err := deploy(c, r); err != nil {
				return err
			}
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

// updateResFromFlags will inspect a resource and apply any flag specific args
func updateResFromFlags(c *cli.Context, r *ObjectResource) error {
	// Add create only option where applicable
	if c.IsSet(FlagCreateOnly) {
		r.CreateOnly = true
		return nil
	}
	// Supports specifying kind/names
	if c.IsSet(FlagCreateOnlyResources) {
		if len(c.StringSlice(FlagCreateOnlyResources)) > 0 {
			for _, resString := range c.StringSlice(FlagCreateOnlyResources) {
				resParts := strings.Split(resString, "/")
				if len(resParts) != 2 {
					return fmt.Errorf(
						"invalid resource type %s, expecting kind/name", resString)
				}
				// Is this the resource we are looking for?
				if strings.ToLower(r.Kind) == strings.ToLower(resParts[0]) &&
					strings.ToLower(r.Name) == strings.ToLower(resParts[1]) {
					r.CreateOnly = true
				}
			}
		}
	}
	return nil
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

	exists := false
	if r.CreateOnly {
		var err error
		exists, err = checkResourceExist(c, r)
		if err != nil {
			return fmt.Errorf(
				"problem checking if resource %s/%s exists", r.Kind, r.Name)
		}
		if exists {
			log.Printf(
				"skipping deploy for resource (%s/%s) marked as create only.",
				r.Kind,
				r.Name)
			return nil
		}
	}

	name := r.Name
	command := "apply"
	if c.Bool(FlagReplace) {
		command = "replace"
	}

	if r.GenerateName != "" {
		name = r.GenerateName
		command = "create"
	}

	logDebug.Printf("about to deploy resource %s/%s (from file:%q)", r.Kind, name, r.FileName)
	args := []string{command, "-f", "-"}
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

	go func() {
		defer stdin.Close()
		stdin.Write(r.Template)
	}()

	logInfo.Printf("deploying %s/%s", strings.ToLower(r.Kind), r.Name)
	if err = cmd.Run(); err != nil {
		if errbuf.Len() > 0 {
			return fmt.Errorf(errbuf.String())
		}
		return err
	}
	logInfo.Print(outbuf.String())

	if r.GenerateName != "" {
		//This gets the generated resource name from the output
		resourceName := strings.TrimSuffix(outbuf.String(), " created\n")
		r.Name = strings.Split(resourceName, "/")[1]
	}

	if isWatchableResouce(r) {
		return watchResource(c, r)
	}
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
	var availableResourceCount int32
	var unavailableResourceCount int32

	for {
		select {
		case <-timeout:
			return fmt.Errorf("%s rolling update %q timed out after %s", r.Kind, r.Name, c.Duration("timeout").String())
		case <-ticker.C:
			r.DeploymentStatus = DeploymentStatus{}

			// Retry on error until max retries is met
			for attempt := 0; attempt < MaxHealthcheckRetries; attempt++ {
				if err := updateResourceStatus(c, r); err != nil {

					// Return error on final try
					if attempt == (MaxHealthcheckRetries - 1) {
						return err
					}

					// Sleep between retries
					time.Sleep(HealthCheckSleepDuration)

				} else {
					break
				}
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
				unavailableResourceCount = r.ObjectSpec.Replicas - r.DeploymentStatus.ReadyReplicas

			case "DaemonSet":
				if (r.DeploymentStatus.DesiredNumberScheduled == r.DeploymentStatus.NumberAvailable) &&
					(r.DeploymentStatus.UpdatedNumberScheduled == r.DeploymentStatus.DesiredNumberScheduled) {
					ready = true
				}
				availableResourceCount = r.DeploymentStatus.NumberAvailable
				unavailableResourceCount = r.DeploymentStatus.DesiredNumberScheduled - r.DeploymentStatus.UpdatedNumberScheduled

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

func checkResourceExist(c *cli.Context, r *ObjectResource) (bool, error) {
	args := []string{"get", r.Kind + "/" + r.Name, "-o", "custom-columns=:.metadata.name", "--no-headers"}
	cmd, err := newKubeCmd(c, args)
	if err != nil {
		return false, err
	}
	stderr, _ := cmd.StderrPipe()
	stdout, _ := cmd.StdoutPipe()
	if err := cmd.Start(); err != nil {
		logDebug.Printf("error starting kubectl: %s", err)
		return false, err
	}
	data, _ := ioutil.ReadAll(stdout)
	if err := cmd.Wait(); err != nil {
		logDebug.Printf("error with kubectl: %s", err)
		errData, _ := ioutil.ReadAll(stderr)
		if strings.Contains("NotFound", string(errData[:])) {
			return false, nil
		}
		return false, err
	}
	if strings.TrimSpace(string(data[:])) == r.Name {
		return true, nil
	} else {
		return false, nil
	}
}

func newKubeCmd(c *cli.Context, args []string) (*exec.Cmd, error) {
	return newKubeCmdSub(c, args, false)
}

func newKubeCmdSub(c *cli.Context, args []string, subCommand bool) (*exec.Cmd, error) {

	kube := "kubectl"
	if c.IsSet("namespace") {
		args = append([]string{"--namespace=" + c.String("namespace")}, args...)
	}
	if c.IsSet("context") {
		args = append([]string{"--context=" + c.String("context")}, args...)
	}
	if c.IsSet("kube-token") {
		args = append([]string{"--token=" + c.String("kube-token")}, args...)
	} else {
		if c.IsSet("kube-username") {
			args = append([]string{"--username=" + c.String("kube-username")}, args...)
		}
		if c.IsSet("kube-password") {
			args = append([]string{"--password=" + c.String("kube-password")}, args...)
		}
	}
	if c.IsSet(FlagCaData) {
		if err := createCertificateAuthority(c.String(FlagCaFile), c.String(FlagCaData)); err != nil {
			return nil, err
		}
		args = append([]string{"--certificate-authority=" + c.String(FlagCaFile)}, args...)
	}
	if c.IsSet(FlagCa) {
		caFile, err := getCaFileAndDownloadIfRequired(c)
		if err != nil {
			return nil, err
		}
		args = append([]string{"--certificate-authority=" + caFile}, args...)
	}
	if c.IsSet("insecure-skip-tls-verify") {
		args = append([]string{"--insecure-skip-tls-verify"}, args...)
	}
	if c.IsSet("kube-server") {
		args = append([]string{"--server=" + c.String("kube-server")}, args...)
	}

	flags, err := extraFlags(c, subCommand)
	if err != nil {
		return nil, err
	}
	// If we've been asked to replace and we haven't provided the '-- --force'
	// extra args, add it here (a update will fail if the object doesn't exist)
	if c.Bool(FlagReplace) {
		forceSet := false
		for _, flag := range flags {
			if strings.Contains(flag, "--force") {
				forceSet = true
				break
			}
		}
		if !forceSet {
			flags = append(flags, "--force")
		}
	}
	args = append(args, flags...)

	return exec.Command(kube, args...), nil
}

// getCaFileAndDownloadIfRequired will obtain a CA file on disk - if required
func getCaFileAndDownloadIfRequired(c *cli.Context) (string, error) {
	// have we done this already?
	if len(caFile) > 0 {
		return caFile, nil
	}
	ca := c.String(FlagCa)
	// Detect if using a URL scheme
	if uri, _ := url.ParseRequestURI(ca); uri != nil {
		if uri.Scheme == "" {
			// Not a URL, get out of here
			return ca, nil
		}
	}
	// Where should we save the ca?
	if c.IsSet(FlagCaFile) {
		caFile = c.String(FlagCaFile)
	} else {
		// This is used by cleanup
		tmpDir, _ = ioutil.TempDir("", "kd")
		caFile = filepath.Join(tmpDir, "kube-ca.pem")
	}

	// skip download if ca file already exists
	if found, err := FilesExists(caFile); err != nil {
		return "", err
	} else if found {
		logDebug.Printf("ca file (%s) already exists, skipping download from: %s", caFile, ca)
		return caFile, nil
	}

	logDebug.Printf("ca file specified as %s, to download from %s", caFile, ca)
	// download the ca...
	resp, err := grab.Get(caFile, ca)
	if err != nil {
		return "", fmt.Errorf(
			"problem downloading ca from %s:%s", resp.Filename, err)
	}
	return caFile, nil
}

// extraFlags will parse out the -- args portion
func extraFlags(c *cli.Context, subCommand bool) ([]string, error) {
	var a []string

	if c.NArg() < 1 {
		return a, nil
	}

	if c.Args()[0] == "--" {
		return c.Args()[1:], nil
	}
	// When we are called from a sub command we don't want the sub command bits
	if subCommand {
		return a, nil
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
