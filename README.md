# kd - Kubernetes resources deployment tool

[![Build Status](https://travis-ci.org/UKHomeOffice/kd.svg?branch=master)](https://travis-ci.org/UKHomeOffice/kd) [![Docker Repository on Quay](https://quay.io/repository/ukhomeofficedigital/kd/status "Docker Repository on Quay")](https://quay.io/repository/ukhomeofficedigital/kd)

This is a very minimalistic tool for deploying kubernetes resources.

## Features

- Go template engine support
- Supports any kubernetes resource type
- Polls deployment resources for completion


## Getting Started

There is only requirement and that is a kubectl binary in your `${PATH}`. You
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
deployment "nginx" created
"nginx" deployment in progress: 3 out of 5 replicas ready..
"nginx" deployment is complete: 5 out of 5 replicas ready.
```


## Configuration

Configuration can be provided via cli flags and arguments as well as
environment variables.

```bash
$ kd --help

NAME:
   kd - simple kubernetes resources deployment tool

USAGE:
   kd [global options] command [command options] [arguments...]
   
AUTHOR(S):
   Vaidas Jablonskis <jablonskis@gmail.com> 
   
COMMANDS:
     help, h  Shows a list of commands or help for one command

GLOBAL OPTIONS:
   --insecure-skip-tls-verify           if true, the server's certificate will not be checked for validity [$KD_INSECURE_SKIP_TLS_VERIFY, $PLUGIN_INSECURE_SKIP_TLS_VERIFY]
   --kube-server URL, -s URL            kubernetes api server URL [$KUBE_SERVER, $PLUGIN_KUBE_SERVER]
   --kube-token TOKEN, -t TOKEN         kubernetes auth TOKEN [$KUBE_TOKEN, $PLUGIN_KUBE_TOKEN]
   --context CONTEXT, -c CONTEXT        kube config CONTEXT [$KUBE_CONTEXT, $PLUGIN_CONTEXT]
   --namespace NAMESPACE, -n NAMESPACE  kubernetes NAMESPACE [$KUBE_NAMESPACE, $PLUGIN_NAMESPACE]
   --file value, -f value               a list of kubernetes resources FILE [$KD_FILES, $PLUGIN_FILES]
   --retries value                      deployment status check retries. Sleep 30s between each check (default: 10) [$RETRIES, $PLUGIN_RETRIES]
   --help, -h                           show help
   --version, -v                        print the version
```


## Build

Dependencies are located in the vendor directory and managed using
[govendor](https://github.com/kardianos/govendor) cli tool.

```
go test -v -cover

mkdir -p bin
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags "-X main.Version=dev+git" -o bin/kd_linux_amd64
```


## Release process

Push / Merge to master will produce a docker
[image](https://quay.io/repository/ukhomeofficedigital/kd?tab=tags) with a tag `latest`.

To create a new release, just create a new tag off master.


## Contributing

We welcome pull requests. Please check existing issues and PRs before submitting a patch.


## Author

Vaidas Jablonskis [vaijab](https://github.com/vaijab)

