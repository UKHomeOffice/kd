# kd - Kubernetes resources deployment tool

[![Build Status](https://travis-ci.org/UKHomeOffice/kd.svg?branch=master)](https://travis-ci.org/UKHomeOffice/kd) [![Docker Repository on Quay](https://quay.io/repository/ukhomeofficedigital/kd/status "Docker Repository on Quay")](https://quay.io/repository/ukhomeofficedigital/kd)

This is a very minimalistic tool for deploying kubernetes resources.

## Features

- Go template engine support
- Supports any kubernetes resource type
- Polls deployment resources for completion
- Polls statefulset resources (only with updateStrategy type set to [RollingUpdates](https://kubernetes.io/docs/tutorials/stateful-application/basic-stateful-set/#rolling-update)).

## Running with Docker
Note that kd can be run with docker, [check here for the latest image tags](https://quay.io/repository/ukhomeofficedigital/kd?tab=tags)

```bash
docker run quay.io/ukhomeofficedigital/kd:latest --help
```
## Installation

Please download the required binary file from the [releases page](https://github.com/UKHomeOffice/kd/releases)

For Mac Users - Please download release `kd_darwin_amd64` and run the following commands. These will ensure the binary is renamed to 'kd', it's also in your `${PATH}`, and that you have permissions to run it on your system.
```bash
mv ~/Downloads/kd_darwin_amd64 /usr/local/bin/kd
chmod u+x /usr/local/bin/kd
```

## Getting Started

The is only requirement and that is a kubectl binary in your `${PATH}`. You
can use the docker image or download the binary for your OS from
[releases](https://github.com/UKHomeOffice/kd/releases) page.

First, let's create a simple deployment template. Templating data comes from
the environment, so in this example we'll use `NGINX_IMAGE_TAG` environment
variable to set nginx image tag.

Create a `nginx-deployment.yaml` with the following content:

```yaml
---
apiVersion: extensions/v1beta1
kind: Deployment
metadata:
  name: nginx
spec:
  replicas: 5
  template:
    metadata:
      labels:
        name: nginx
    spec:
      containers:
        - name: nginx
          image: nginx:{{.NGINX_IMAGE_TAG}}
          ports:
            - containerPort: 80
          resources:
            limits:
              cpu: "0.1"
          livenessProbe:
            httpGet:
              path: /
              port: 80
            initialDelaySeconds: 10
            timeoutSeconds: 1
```

```bash
$ export NGINX_IMAGE_TAG=1.11-alpine
$ kd --context=mykube --namespace=testing --file nginx-deployment.yaml
[INFO] 2016/09/21 14:06:37 main.go:153: deploying deployment/nginx
[INFO] 2016/09/21 14:06:38 main.go:157: deployment "nginx" submitted
[INFO] 2016/09/21 14:06:41 main.go:194: deployment "nginx" in progress. Unavailable replicas: 5.
[INFO] 2016/09/21 14:06:56 main.go:194: deployment "nginx" in progress. Unavailable replicas: 5.
[INFO] 2016/09/21 14:07:11 main.go:190: deployment "nginx" is complete. Available replicas: 5
```

You can fail an ongoing deployment if there's been a new deployment by adding `--fail-superseded` flag.

### Run command

You can run kubectl with the support of the same flags and environment variables
that kd supports to simplify scripted deployments.

```bash
$ export KUBE_NAMESPACE=testland
$ kd run get po -l app=myapp -o custom-columns=:.metadata.name --no-headers
```

## Templating

You can add the flag --debug-templates to render templates at run time.
Check the examples folder for more info.

[Sprig](https://masterminds.github.io/sprig/) is used to add templating functions.

To preserve backwards compatibility (parameter order) the following functions
 still use the [golang strings libraries](https://golang.org/pkg/strings/):

- contains
- hasPrefix
- hasSuffix
- [split](#split)

The function for including files is also preserved from before:

- [file](#file)

### split

`split` function is go's `strings.Split()`, it returns a `[]string`. A range function
can also be used to iterate over returned list.

```yaml
# split.yaml
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: list
data:
  foo:
{{ range split .LIST "," }}
    - {{ . }}
    {{- end -}}
```

```
$ export LIST="one,two,three"
$ ./kd -f split.yaml --dryrun --debug-templates
[INFO] 2017/10/18 15:08:09 main.go:241: deploying configmap/list
[INFO] 2017/10/18 15:08:09 main.go:248: apiVersion: v1
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: list
data:
  foo:
    - one
    - two
    - three
```

### file

`file` function will locate and render a configuration file from your repo. A full path will need to be specified, you can run this in drone by using `workspace:` and a base directory (http://docs.drone.io/workspace/#app-drawer). Here's an example:

```yaml
# file.yaml
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: list
data:
  foo:
{{ file .BAR | indent 4}}
```

```
$ cat <<EOF > config.yaml
- one
- two
- three
EOF
$ export BAR=${PWD}/config.yaml
$ ./kd -f file.yaml --dryrun --debug-templates
[INFO] 2017/10/18 15:08:09 main.go:241: deploying configmap/list
[INFO] 2017/10/18 15:08:09 main.go:248: apiVersion: v1
apiVersion: v1
kind: ConfigMap
metadata:
  name: list
data:
  foo:
    - one
    - two
    - three
```

## Configuration

Configuration can be provided via cli flags and arguments as well as
environment variables.

It supports end of flags `--` parameter, any flags or arguments that are
specified after `--` will be passed onto kubectl.

```bash
$ kd --help
NAME:
   kd - simple kubernetes resources deployment tool

USAGE:
   kd [global options] command [command options] [arguments...]

AUTHOR:
   Vaidas Jablonskis <jablonskis@gmail.com>

COMMANDS:
     run      run [kubectl args] - runs kubectl supporting kd flags / environment options
     help, h  Shows a list of commands or help for one command

GLOBAL OPTIONS:
   --debug                              debug output [$DEBUG, $PLUGIN_DEBUG]
   --debug-templates                    debug template output [$DEBUG_TEMPLATES, $PLUGIN_DEBUG_TEMPLATES]
   --dryrun                             if true, kd will exit prior to deployment [$DRY_RUN]
   --insecure-skip-tls-verify           if true, the server's certificate will not be checked for validity [$INSECURE_SKIP_TLS_VERIFY, $PLUGIN_INSECURE_SKIP_TLS_VERIFY]
   --kube-server URL, -s URL            kubernetes api server URL [$KUBE_SERVER, $PLUGIN_KUBE_SERVER]
   --kube-token TOKEN, -t TOKEN         kubernetes auth TOKEN [$KUBE_TOKEN, $PLUGIN_KUBE_TOKEN]
   --config value                       Env file location [$CONFIG_FILE, $PLUGIN_CONFIG_FILE]
   --context CONTEXT, -c CONTEXT        kube config CONTEXT [$KUBE_CONTEXT, $PLUGIN_CONTEXT]
   --namespace NAMESPACE, -n NAMESPACE  kubernetes NAMESPACE [$KUBE_NAMESPACE, $PLUGIN_KUBE_NAMESPACE]
   --fail-superseded                    fail deployment if it has been superseded by another deployment. WARNING: there are some bugs in kubernetes. [$FAIL_SUPERSEDED, $PLUGIN_FAIL_SUPERSEDED]
   --certificate-authority PATH         the path to a file containing the CA for kubernetes API PATH [$KUBE_CERTIFICATE_AUTHORITY, $PLUGIN_KUBE_CERTIFICATE_AUHORITY]
   --certificate-authority-data PATH    the certificate authority data for the kubernetes API PATH [$KUBE_CERTIFICATE_AUTHORITY_DATA, $PLUGIN_KUBE_CERTIFICATE_AUHORITY_DATA]
   --certificate-authority-file value   the path to file the certificate authority file from certifacte-authority-data option (default: "/tmp/kube-ca.pem")
   --file PATH, -f PATH                 the path to a file or directory containing kubernetes resource/s PATH [$FILES, $PLUGIN_FILES]
   --timeout TIMEOUT, -T TIMEOUT        the amount of time to wait for a successful deployment TIMEOUT (default: 3m0s) [$TIMEOUT, $PLUGIN_TIMEOUT]
   --check-interval INTERVAL            deployment status check interval INTERVAL (default: 1s) [$CHECK_INTERVAL, $PLUGIN_CHECK_INTERVAL]
   --help, -h                           show help
   --version, -v                        print the version

```

## Build

Dependencies are located in the vendor directory and managed using
[glide](https://github.com/Masterminds/glide/blob/master/README.md) cli tool.
Install glide e.g. `curl https://glide.sh/get | sh` then use it to *install*
vendored libs and then build e.g:
```
glide install
go test -v -cover

mkdir -p bin
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags "-X main.Version=dev+git" -o bin/kd_linux_amd64
```

You can also build `kd` in a docker container:

```bash
docker run --rm -v $PWD:/go/src/github.com/UKHomeOffice/kd -w /go/src/github.com/UKHomeOffice/kd -ti golang:1.6 bash
cd /go/src/UKHomeOffice/kd
go get
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags "-X main.Version=dev+git" -o bin/kd_linux_amd64
```

## Release process

Push / Merge to master will produce a docker
[image](https://quay.io/repository/ukhomeofficedigital/kd?tab=tags) with a tag `latest`.

To create a new release, just create a new tag off master.


## Contributing

We welcome pull requests. Please raise an issue to discuss your changes before
submitting a patch.


## Author

Vaidas Jablonskis [vaijab](https://github.com/vaijab)
