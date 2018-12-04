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

### Replace

kd will use the `apply` verb to create / update resources which is [appropriate
in most cases](https://kubernetes.io/docs/concepts/cluster-administration/manage-deployment/#in-place-updates-of-resources).

The flag `--replace` can be used to override this behaviour and may be useful in
some very specific scenarios however the result can be a [disruptive update](https://kubernetes.io/docs/concepts/cluster-administration/manage-deployment/#disruptive-updates)
if extra kubectl flags are applied (such as `-- --force`). Additionally, the last-applied-configuration is not saved when using this flag.

#### Cronjobs

When a cronjob object is created and only updated, any old jobs will continue and some
fields are immutable, so use of the replace command and force option may be required.

```bash
# The cronjob resource does not yet exist, and so a create action is performed
$ kd --replace -f cronjob.yml -- --force
[INFO] 2018/08/07 22:54:00 main.go:724: resource does not exist, dropping --force flag for create action
[INFO] 2018/08/07 22:54:00 main.go:466: deploying cronjob/etcd-backup
[INFO] 2018/08/07 22:54:00 main.go:473: cronjob "etcd-backup" created

# The resource now exists, so a kubectl replace is performed with the extra --force arg
$ kd --replace -f cronjob.yml -- --force
[INFO] 2018/08/07 22:54:02 main.go:466: deploying cronjob/etcd-backup
[INFO] 2018/08/07 22:54:03 main.go:473: cronjob "etcd-backup" deleted
cronjob "etcd-backup" replaced
```

#### Large Objects

As an apply uses 'patch' internally, there is a limit to the size of objects
that can be updated this way and you may receive an error such as: `metadata.annotations: Too long: must have at most 262144 characters`

Below is an example for updating a large ConfigMap:
```bash
# 859KB ConfigMap resource
$ kd --replace -f configmap.yaml
[INFO] 2018/08/07 23:02:39 main.go:466: deploying configmap/bundle
[INFO] 2018/08/07 23:02:40 main.go:473: configmap "bundle" created

$ kd --replace -f configmap.yaml
[INFO] 2018/08/07 23:02:41 main.go:466: deploying configmap/bundle
[INFO] 2018/08/07 23:02:42 main.go:473: configmap "bundle" replaced
```

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

kd specific template functions:

- [file](#file)
- [fileWith](#fileWith)
- [secret](#secret)
- [k8lookup](#k8lookup)

Extra template functions (from helm):

- [strvals package](https://github.com/helm/helm/blob/master/pkg/strvals/parser.go)

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

### fileWith

`fileWith` function will locate and render a configuration file from your repo with additional values specified as a Sprig dict. A full path will need to be specified, you can run this in drone by using `workspace:` and a base directory (http://docs.drone.io/workspace/#app-drawer). Here's an example:

```yaml
# file.yaml
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: list
data:
  foo:
{{ fileWith .BAR (dict "FIRST" "one") | indent 4}}
```

```
$ cat <<EOF > config.yaml
- {{ .FIRST }}
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

### secret

`secret` function generates a secret given the parameters `type` and `length`.
Supported types are:

- alphanum
- mysql
- yaml

**NOTE** a secret generated will automatically be set to `create-only` and will
not be updated for every deploy.

```yaml
# secret.yaml
---
apiVersion: v1
kind: Secret
metadata:
  name: test
data:
  # generate a mysql safe password of 20 chars
  password: {{ secret "mysql" 20 }}
```

```bash
$ ./kd -f ./test/secret.yaml --dryrun --debug-templates
[DEBUG] 2018/07/17 15:10:50 main.go:240: about to open file:./test/secret.yaml
[DEBUG] 2018/07/17 15:10:50 main.go:260: parsing file:./test/secret.yaml
[INFO] 2018/07/17 15:10:50 main.go:282: Template:
apiVersion: v1
kind: Secret
metadata:
  name: test
type: Opaque
data:
  username: bob
  # Create a secret suitable for mysql of 20 chars
  password: fD1wS2kzTUVVNUdJcDxGWkhedmQ=
```

If you are creating a kubernetes secret and need the content to be automatically base64 encoded, you can do the following:
```yml
# secret.yml
apiVersion: v1
kind: Secret
metadata:
  name: test
data:
  # read the contents of the "MY_HOSTNAME" environment variable and base64 encode it
  hostname: {{ .MY_HOSTNAME | b64enc }}
  # base64 encode the provided string
  username: {{ "my-username" | b64enc }}
```

### k8lookup

`k8lookup` function allows retrieval of an Kubernetes object value using the parameters:
- `kind` - a Kubernetes object kind e.g. `pv` or `PersistentVolume`
- `name` - an object name e.g. `sysdig-mysql-a`
- `path` - a object path reference e.g. `.spec.capacity.storage`

Example:

With manually provisioned storage (e.g. iSCSI or NFS) a PV is typically managed
using a separate repository. Using lookup, we can discover the appropriate
storage size for a given cluster automatically:

```
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  labels:
    name: sysdig-galera
  name: data-sysdig-galera-0
spec:
  accessModes:
  - ReadWriteOnce
  resources:
    requests:
      storage: {{ k8lookup "pv" "sysdig-mysql-a" ".spec.capacity.storage" }}
  selector:
    matchLabels:
      name: sysdig-mysql
  storageClassName: manual
```

## Configuration

Configuration can be provided via cli flags and arguments as well as
environment variables.

### Config

`--config` use of a .env file see [github.com/joho/godotenv](https://github.com/joho/godotenv/blob/master/README.md)

### Config Data

`--config-data` can be specified to facilitate structured yaml data in templates. It has two forms:

1. `--config-data Scope=file.yaml` - data from file available at `.Scope.rootkey.subkey`
2. `--config-data  file.yaml` - data from file available at `.rootkey.subkey`

E.g. A deployment using source copied from a simple helm chart source:

```
kd --config-data Chart=./helm/simple-app/Chart.yaml \
   --config-data Values=./helm/simple-app/values.yaml \
   --allow-missing \
   --file ./helm/simple-app/templates/
```

### Kubectl flags

It supports end of flags `--` parameter, any flags or arguments that are
specified after `--` will be passed onto kubectl.

```
$ kd --help
NAME:
   kd - simple kubernetes resources deployment tool

USAGE:
   kd [global options] command [command options] [arguments...]

VERSION:
   v1.10.7

AUTHOR:
   Vaidas Jablonskis <jablonskis@gmail.com>

COMMANDS:
     run      run [kubectl args] - runs kubectl supporting kd flags / environment options
     help, h  Shows a list of commands or help for one command

GLOBAL OPTIONS:
   --debug                                debug output [$DEBUG, $PLUGIN_DEBUG]
   --debug-templates                      debug template output [$DEBUG_TEMPLATES, $PLUGIN_DEBUG_TEMPLATES]
   --dryrun                               if true, kd will exit prior to deployment [$DRY_RUN]
   --delete                               instead of applying the resources we are deleting them
   --insecure-skip-tls-verify             if true, the server's certificate will not be checked for validity [$INSECURE_SKIP_TLS_VERIFY, $PLUGIN_INSECURE_SKIP_TLS_VERIFY]
   --kube-config-data value               Kubernetes config file data [$KUBE_CONFIG_DATA, $PLUGIN_KUBE_CONFIG_DATA]
   --kube-server URL, -s URL              kubernetes api server URL [$KUBE_SERVER, $PLUGIN_KUBE_SERVER]
   --kube-token TOKEN, -t TOKEN           kubernetes auth TOKEN [$KUBE_TOKEN, $PLUGIN_KUBE_TOKEN]
   --kube-username USERNAME, -u USERNAME  kubernetes auth USERNAME [$KUBE_USERNAME, $PLUGIN_KUBE_USERNAME]
   --kube-password PASSWORD, -p PASSWORD  kubernetes auth PASSWORD [$KUBE_PASSWORD, $PLUGIN_KUBE_PASSWORD]
   --config value                         Env file location [$CONFIG_FILE, $PLUGIN_CONFIG_FILE]
   --config-data value                    Config data e.g. --config-data=Chart=./Chart.yaml [$KD_CONFIG_DATA, $PLUGIN_KD_CONFIG_DATA]
   --create-only                          only create resources (do not update, skip if exists). [$CREATE_ONLY, $PLUGIN_CREATE_ONLY]
   --create-only-resource value           only create specified resources e.g. 'kind/name' (do not update, skip if exists). [$CREATE_ONLY_RESOURCES, $PLUGIN_CREATE_ONLY_RESOURCES]
   --replace                              use replace instead of apply for updating objects [$KUBE_REPLACE, $PLUGIN_KUBE_REPLACE]
   --context CONTEXT, -c CONTEXT          kube config CONTEXT [$KUBE_CONTEXT, $PLUGIN_CONTEXT]
   --namespace NAMESPACE, -n NAMESPACE    kubernetes NAMESPACE [$KUBE_NAMESPACE, $PLUGIN_KUBE_NAMESPACE]
   --fail-superseded                      fail deployment if it has been superseded by another deployment. WARNING: there are some bugs in kubernetes. [$FAIL_SUPERSEDED, $PLUGIN_FAIL_SUPERSEDED]
   --certificate-authority PATH           the path (or URL) to a file containing the CA for kubernetes API PATH [$KUBE_CERTIFICATE_AUTHORITY, $PLUGIN_KUBE_CERTIFICATE_AUTHORITY]
   --certificate-authority-data PATH      the certificate authority data for the kubernetes API PATH [$KUBE_CERTIFICATE_AUTHORITY_DATA, $PLUGIN_KUBE_CERTIFICATE_AUTHORITY_DATA]
   --certificate-authority-file value     the path to save certificate authority data to when data or a URL is specified (default: "/tmp/kube-ca.pem") [$KUBE_CERTIFICATE_AUTHORITY_FILE, $PLUGIN_KUBE_CERTIFICATE_AUTHORITY_FILE]
   --file PATH, -f PATH                   the path to a file or directory containing kubernetes resources PATH [$FILES, $PLUGIN_FILES]
   --timeout TIMEOUT, -T TIMEOUT          the amount of time to wait for a successful deployment TIMEOUT (default: 3m0s) [$TIMEOUT, $PLUGIN_TIMEOUT]
   --check-interval INTERVAL              deployment status check interval INTERVAL (default: 1s) [$CHECK_INTERVAL, $PLUGIN_CHECK_INTERVAL]
   --allow-missing                        if true, missing variables will be replaced with <no value> instead of generating an error [$ALLOW_MISSING]
   --help, -h                             show help
   --version, -v                          print the version
```

## Build

To build kd just run `make`.

You can also build `kd` in a docker container:

```bash
docker run --rm -v $PWD:/go/src/github.com/UKHomeOffice/kd -w /go/src/github.com/UKHomeOffice/kd -ti golang:1.10 make
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
