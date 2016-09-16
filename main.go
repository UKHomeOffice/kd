package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"strings"
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
		cli.StringSliceFlag{
			Name:   "file, f",
			Usage:  "list of kubernetes resources FILE",
			EnvVar: "FILES,PLUGIN_FILES",
		},
		cli.IntFlag{
			Name:   "retries",
			Usage:  "number of deployment status check retries",
			EnvVar: "RETRIES,PLUGIN_RETRIES",
			Value:  10,
		},
		cli.DurationFlag{
			Name:   "check-interval",
			Usage:  "deployment status check interval",
			EnvVar: "CHECK_INTERVAL,PLUGIN_CHECK_INTERVAL",
			Value:  15 * time.Second,
		},
	}

	app.Action = run
	app.Run(os.Args)
}

func run(c *cli.Context) error {
	if len(c.StringSlice("file")) == 0 {
		logError.Print("no kubernetes resource files specified")
		return cli.NewExitError("", 1)
	}
	// Check if all files exist first - fail early
	for _, fn := range c.StringSlice("file") {
		if _, err := os.Stat(fn); err != nil {
			logError.Println(err)
			return cli.NewExitError("", 1)
		}
	}

	for _, fn := range c.StringSlice("file") {
		// TODO: check if `-f` is a directory and expand all files in it
		f, err := os.Open(fn)
		if err != nil {
			logError.Println(err)
			return cli.NewExitError("", 1)
		}
		defer f.Close()

		resource := ObjectResource{FileName: fn}
		if err := render(&resource, envToMap(), c.Bool("debug")); err != nil {
			logError.Println(err)
			return cli.NewExitError("", 1)
		}
		if err := yaml.Unmarshal(resource.Template, &resource); err != nil {
			logError.Println(err)
			return cli.NewExitError("", 1)
		}
		if err := deploy(c, &resource); err != nil {
			logError.Println(err)
			return cli.NewExitError("", 1)
		}
	}

	return nil
}

func deploy(c *cli.Context, r *ObjectResource) error {
	args := []string{"apply", "-f", "-"}
	cmd := newKubeCmd(c, args)
	if c.Bool("debug") {
		logDebug.Printf("kubectl arguments: %q", strings.Join(cmd.Args, " "))
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}

	stdin.Write(r.Template)
	stdin.Close()
	if err != nil {
		return err
	}
	logInfo.Printf("deploying %s/%s", strings.ToLower(r.Kind), r.Name)
	if err = cmd.Run(); err != nil {
		return err
	}
	logInfo.Printf("%s %q submitted", strings.ToLower(r.Kind), r.Name)
	if r.Kind != "Deployment" {
		return nil
	}

	// TODO: should use a proper timeout instead of retries
	retries := c.Int("retries")
	attempt := 1
	if err := updateDeploymentStatus(c, r); err != nil {
		return err
	}
	// If this is a new deployment, Replicas and UpdatedReplicas count will
	// be 0, so we want to wait and retry
	if r.DeploymentStatus.Replicas == 0 && r.DeploymentStatus.UpdatedReplicas == 0 {
		if c.Bool("debug") {
			logDebug.Printf("new deployment, sleeping 3 seconds")
		}
		time.Sleep(3 * time.Second)
	}

	og := r.DeploymentStatus.ObservedGeneration
	for {
		r.DeploymentStatus = DeploymentStatus{}
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
		if c.Bool("debug") {
			logDebug.Printf("sleeping for %q", c.Duration("check-interval"))
		}
		time.Sleep(c.Duration("check-interval"))
		attempt++
		if attempt > retries {
			return fmt.Errorf("deployment %q failed. Max retries reached", r.Name)
		}

		// Fail the deployment in case another deployment has started
		if c.Bool("fail-superseded") {
			if og != r.DeploymentStatus.ObservedGeneration {
				return fmt.Errorf("deployment failed. It has been superseded by another deployment")
			}
		}
	}
}

func updateDeploymentStatus(c *cli.Context, r *ObjectResource) error {
	args := []string{"get", "deployment/" + r.Name, "-o", "yaml"}
	cmd := newKubeCmd(c, args)
	cmd.Stderr = os.Stderr
	stdout, _ := cmd.StdoutPipe()
	defer stdout.Close()
	if err := cmd.Start(); err != nil {
		return err
	}
	data, _ := ioutil.ReadAll(stdout)
	if err := yaml.Unmarshal(data, r); err != nil {
		return err
	}
	if c.Bool("debug") {
		logDebug.Println("start of deployment yaml received from kubectl")
		fmt.Println(string(data))
		logDebug.Println("end of deployment yaml received from kubectl")
	}
	if err := cmd.Wait(); err != nil {
		return err
	}
	return nil
}

func newKubeCmd(c *cli.Context, args []string) *exec.Cmd {
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
	if c.IsSet("insecure-skip-tls-verify") {
		args = append([]string{"--insecure-skip-tls-verify"}, args...)
	}
	if c.IsSet("kube-server") {
		args = append([]string{"--server=" + c.String("kube-server")}, args...)
	}

	return exec.Command(kube, args...)
}
