package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"time"

	"github.com/urfave/cli"
	"gopkg.in/yaml.v2"
)

// Version is set at compile time, passing -ldflags "-X main.Version=<build version>"
var Version string

func main() {
	app := cli.NewApp()
	app.Name = "kd"
	app.Author = "Vaidas Jablonskis <jablonskis@gmail.com>"
	app.Version = Version
	app.Usage = "simple kubernetes resources deployment tool"

	app.Flags = []cli.Flag{
		cli.BoolFlag{
			Name:   "verbose",
			Usage:  "verbose output",
			EnvVar: "VERBOSE,PLUGIN_VERBOSE",
			Hidden: true,
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
			Usage:  "a list of kubernetes resources FILE",
			EnvVar: "FILES,PLUGIN_FILES",
		},
		cli.IntFlag{
			Name:   "retries",
			Usage:  "deployment status check retries. Sleep 30s between each check",
			EnvVar: "RETRIES,PLUGIN_RETRIES",
			Value:  10,
		},
	}

	app.Action = run
	app.Run(os.Args)
}

func run(c *cli.Context) error {
	if len(os.Args) < 2 {
		cli.ShowAppHelp(c)
	}
	if len(c.StringSlice("file")) == 0 {
		return cli.NewExitError("At least one resource file must be specified.", 1)
	}
	// Check if all files exist first - fail early
	for _, fn := range c.StringSlice("file") {
		if _, err := os.Stat(fn); err != nil {
			return cli.NewExitError(err.Error(), 1)
		}
	}

	for _, fn := range c.StringSlice("file") {
		// TODO: check if `-f` is a directory and expand all files in it
		f, err := os.Open(fn)
		if err != nil {
			return cli.NewExitError(err.Error(), 1)
		}
		defer f.Close()

		resource := ObjectResource{FileName: fn}
		if err := render(&resource, envToMap()); err != nil {
			return cli.NewExitError(err.Error(), 1)
		}
		if err := yaml.Unmarshal(resource.Template, &resource); err != nil {
			return cli.NewExitError(err.Error(), 1)
		}
		if err := deploy(c, &resource); err != nil {
			return cli.NewExitError(err.Error(), 1)
		}
	}

	return nil
}

func deploy(c *cli.Context, r *ObjectResource) error {
	args := []string{"apply", "-f", "-"}
	cmd := newKubeCmd(c, args)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}

	stdin.Write(r.Template)
	stdin.Close()
	if err != nil {
		return err
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err = cmd.Run(); err != nil {
		return err
	}

	if r.Kind != "Deployment" {
		return nil
	}

	// TODO: should use a proper timeout instead of retries
	retries := c.Int("retries")
	attempt := 1
	if err := updateDeploymentStatus(c, r); err != nil {
		return err
	}
	og := r.DeploymentStatus.ObservedGeneration
	for {
		r.DeploymentStatus = DeploymentStatus{}
		if err := updateDeploymentStatus(c, r); err != nil {
			return err
		}

		// If this is a new deployment, r.DeploymentStatus.Replicas count will
		// be 0, so we want to wait and retry
		if r.DeploymentStatus.Replicas == 0 {
			time.Sleep(5 * time.Second)
			continue
		}

		if (r.DeploymentStatus.UnavailableReplicas == 0 || r.DeploymentStatus.AvailableReplicas == r.DeploymentStatus.Replicas) &&
			r.DeploymentStatus.Replicas == r.DeploymentStatus.UpdatedReplicas {
			fmt.Printf("%q deployment is complete. Available replicas: %d.\n",
				r.Name, r.DeploymentStatus.AvailableReplicas)
			return nil
		}
		fmt.Printf("%q deployment in progress. Unavailable replicas: %d.\n",
			r.Name, r.DeploymentStatus.UnavailableReplicas)
		time.Sleep(time.Second * 30)
		attempt++
		if attempt > retries {
			return fmt.Errorf("Deployment failed. Max retries reached.")
		}

		// Fail the deployment in case another deployment has started
		if c.Bool("fail-superseded") {
			if og != r.DeploymentStatus.ObservedGeneration {
				return fmt.Errorf("Deployment failed. It has been superseded by another deployment.")
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
	if err := cmd.Wait(); err != nil {
		return err
	}
	return nil
}

func newKubeCmd(c *cli.Context, args []string) *exec.Cmd {
	kube := "kubectl"
	if c.IsSet("kube-server") || isAnyEnvSet("KUBE_SERVER", "PLUGIN_KUBE_SERVER") {
		args = append(args, "--server="+c.String("kube-server"))
	}
	if c.IsSet("insecure-skip-tls-verify") || isAnyEnvSet("INSECURE_SKIP_TLS_VERIFY", "PLUGIN_INSECURE_SKIP_TLS_VERIFY") {
		args = append(args, "--insecure-skip-tls-verify")
	}
	if c.IsSet("kube-token") || isAnyEnvSet("KUBE_TOKEN", "PLUGIN_KUBE_TOKEN") {
		args = append(args, "--token="+c.String("kube-token"))
	}
	if c.IsSet("context") || isAnyEnvSet("KUBE_CONTEXT", "PLUGIN_KUBE_CONTEXT") {
		args = append(args, "--context="+c.String("context"))
	}
	if c.IsSet("namespace") || isAnyEnvSet("KUBE_NAMESPACE", "PLUGIN_KUBE_NAMESPACE") {
		args = append(args, "--namespace="+c.String("namespace"))
	}
	return exec.Command(kube, args...)
}
