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
			EnvVar: "KD_VERBOSE,PLUGIN_VERBOSE",
			Hidden: true,
		},
		cli.BoolFlag{
			Name:   "insecure-skip-tls-verify",
			Usage:  "if true, the server's certificate will not be checked for validity",
			EnvVar: "KD_INSECURE_SKIP_TLS_VERIFY,PLUGIN_INSECURE_SKIP_TLS_VERIFY",
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
			EnvVar: "KUBE_NAMESPACE,PLUGIN_NAMESPACE",
		},
		cli.StringSliceFlag{
			Name:   "file, f",
			Usage:  "a list of kubernetes resources FILE",
			EnvVar: "KD_FILES,PLUGIN_FILES",
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

		resource := ObjectResource{}
		resource.FileName = fn
		data, err := ioutil.ReadAll(f)
		if err != nil {
			return cli.NewExitError(err.Error(), 1)
		}
		if err := yaml.Unmarshal(data, &resource); err != nil {
			return cli.NewExitError(err.Error(), 1)
		}
		resource.Template = data

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

	if err = render(r, envToMap(), stdin); err != nil {
		return err
	}
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
	for {
		if err := updateDeploymentStatus(c, r); err != nil {
			return err
		}
		if r.DeploymentStatus.Replicas == r.DeploymentStatus.UpdatedReplicas {
			fmt.Printf("%q deployment is complete: %d out of %d replicas ready.\n",
				r.Name, r.DeploymentStatus.UpdatedReplicas, r.DeploymentStatus.Replicas)
			return nil
		}
		fmt.Printf("%q deployment in progress: %d out of %d replicas ready..\n",
			r.Name, r.DeploymentStatus.UpdatedReplicas, r.DeploymentStatus.Replicas)
		time.Sleep(time.Second * 30)
		attempt++
		if attempt > retries {
			return fmt.Errorf("Deployment failed. Max retries reached.")
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
	if c.IsSet("kube-server") {
		args = append(args, "--server="+c.String("kube-server"))
	}
	if c.IsSet("insecure-skip-tls-verify") {
		args = append(args, "--insecure-skip-tls-verify")
	}
	if c.IsSet("kube-token") {
		args = append(args, "--token="+c.String("kube-token"))
	}
	if c.IsSet("context") {
		args = append(args, "--context="+c.String("context"))
	}
	if c.IsSet("namespace") {
		args = append(args, "--namespace="+c.String("namespace"))
	}
	return exec.Command(kube, args...)
}
