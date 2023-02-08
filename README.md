# atkmod: A go library API for module manifests

![Go build](https://github.com/cloud-native-toolkit/atkmod/actions/workflows/go.yml/badge.svg)

> Note: this proof of concept is not dead, but as of Sept 21, 2022 is shelved 
> while some other work. That being said, the module is in use by the `itz` 
> [CLI](https://github.com/cloud-native-toolkit/itzli), which is also a proof 
> of concept. This module includes some code for dealing with the Podman/Docker
> command line and output that is unit tested, and it was better to re-use this 
> than copy and paste it or include it in the CLI outside this module.

The purpose of this project was originally to demonstrate a proof of concept for
defining a "module file" that would be basically a descriptor used in a module's
repository to provide a mechanism for dealing with the module using a
Podman-based, container plugin architecture.

In other words, this was intended to prove an approach to answer the questions:

* *What if I didn't need to care how I got the parameters, or how I even
  deployed the module?*
* *What if I left that up to plugins (in the form of containers that emit
  documented output)?*

## Overview

To accomplish these goals, this project uses a container-based, plugin-style
architecture in which a manifest file defines a deployment lifecycle (see
["Lifecycle Overview"](#lifeycle-overview) for details) for the given module.
This was inspired by build solutions, such as [Drone CI](https://docs.drone.io/)
to a great extent, especially the container-based execution steps.

This project includes an "executor" that simply stages of the lifecycle,
using state to collect error conditions. For each plugin, the container mounts a
local folder as a volume in the container and then executes the entrypoint to
act upon the files in the volume. For example, during the *deploy* stage of the
lifecycle, the itz-plugins/terrform-base plugin will use Terraform to apply the
`main.tf` file and print the command's standard out to the plugin's standard out
to be captured and used by whatever consumes this library.

Other plugins should print information to standard out for their stage in the
lifecycle, such as the list-variables and get-state. See more in the
documentation for the lifecycles.

### What this is not

Notice, however, that other than the required input and output variables for the
module that other dependency information is left out. That is by design. For
the sake of [separation of concerns](https://en.wikipedia.org/wiki/Separation_of_concerns),
a module's dependencies and how a particular module should be installed and
dealt with are two separate concerns. It's the opinionated stance of this library
that it is only concerned with managing a module through its lifecycle via
plugins. Dependency management--including downloading and resolving
dependencies--are the responsibility of some other component.

## Lifeycle Overview

The *lifecycle* is a set of three stages for the deployment: *pre_deploy*,
*deploy*, and *post_deploy*. These stages are intended to be called in order
listed, serially, with each stage completing successfully before continuing on to
the next stage. That means that, by default, an error in *pre_deploy* stage means
that the executor will not run the *deploy* and *post_deploy* stages.

Additionally, there are three *hooks* meta information about the lifecycle:
*list*, *validate*, and *get_state*. These are called various times during the
execution of the lifecycle should output information to standard output in a
specific format and should also accept information via standard input in a
specific format. You can read more on the expected formats in the documentation
for the lifecycle hook. Because the hooks should get information about the
module and the environment, they are covered first in this documentation.

### Hook: get_state

The *get_state* hook is called by the executor to get the current *state* of the
module. For each module, that could mean something slightly different. For
example, for a module that configures basic networking that might mean returning
the IP addresses of the existing network. Therefore, aside from the `health`
element, the state returned as a response can be in any form returned in
properly-formatted JSON within the `data` element. As example is show here:

```json
{
  "health": {
    "status": "DEPLOYED",
    "lifecycle": {
      "pre_deploy": "SUCCESS",
      "deploy": "SUCCESS",
      "post_deploy": "SUCCESS"
    }
  },
  "data": {}
}
```

This state is passed along to the lifecycle plugins as STDIN. They can use it or
ignore it--that's the flexibility of these plugins. The `data` element here
should contain various state data important to the module. For example, if the
module is a base VPC network in AWS, this state data may include the VPC and
subnet IDs, IP addresses, etc.

Since `health` is reserved, a `status` of "DEPLOYED" should mean the module has
been successfully deployed. In the `lifecycle` elements, each name maps exactly
to the name of the stage in the lifecycle. This is intended to provide the
capacity for retry or rollback logic, where a status of "SUCCESS" for the
*deploy* stage of the lifecycle will be skipped by default. If the status were
something else, the *deploy* stage may be retried, depending on the
implementation of the plugin.

### Hook: list

The responsibility of the *list* hook is to provide information about the module
to the executor; most importantly is the list of input variables expected by the
executor.

Implementations of this hook can vary from reading input files, such as .tfvars
or tfinput files or source files.

> Note: Output information should be included in the *get_state* hook.

Listed here is an example of the output from list:

```json
{
  "variables": [
    {
      "name": "TF_VAR_cluster_api_url",
      "default": "https://mycluster.example.com"
    },
    {
      "name": "TF_VAR_github_repo_url",
      "default": "https://github.com/some-repo"
    }
  ]
}
```

### Hook: validate

The *validate* hook provides a means to validate state of the module before
executing any of the lifecycle stages. The validate hook should take the
`variables` input as STDIN and return a status like the following JSON:

```json
{
  "status": "OK",
  "messages": []
}
```

Shown here is an example of an error status that provides a meaningful error
message to the caller:

```json
{
  "status": "ERROR",
  "messages": [
    "Variable 'TF_VAR_cluster_api' is invalid."
  ]
}
```

The *validate* hook should be designed to validate single variables as
well as the entire variable set or more and should be idempotent. This is so
callers can call validate a single variable, such as in the case of an interactive
prompt validating the input of a question before proceeding to the next, or
validation of several variables at once in the case of a source or .env file
that contains many input variables.

### Stage: pre_deploy

The *pre_deploy* stage is used to initialize the workspace (working volume) to
the state that it should be in prior to proceeding to the *deploy* stage.
Plugin implementations for this stage could download any dependencies, run a
command such as `tf plan`

Like the rest of the plugins, errors should result in a non-zero exit status
from the container execution as well as some error messages written to STDOUT.
See "[Handling errors](#handling-errors)" for more information about the
error message envelope.

### Stage: deploy

The *deploy* stage is where the actual deployment of the module in the workspace
is performed by the execution engine. In Terraform terms, this is where the
Terraform plugin executes the `tf apply` command or the Ansible plugin executes
the `ansible-playbook` or the CloudFormation plugin executes the 
`aws cloudformation create-stack` command or `oc apply -f` for an OpenShift
application by default.

Regardless of the implementation, the plugin should actually deploy the module,
whatever that means for the given module.

### Stage: post_deploy

The *post_deploy* stage is where a plugin can perform cleanup, validation, 
writing state, etc., of the module.

## The module manifest file

Examples of the module manifest file are best viewed in the *test/examples*
directory, because those are the files that are run against unit tests and
therefore verified against actual code. But, for convenience, an example is
shown here:

```yaml
# The apiVersion of the file. Supported values for the apiVersion are currently
# only v1alpha1. Any other value will cause an error when the file is being 
# loaded. itzcli is the namespace.
apiVersion: itzcli/v1alpha1
# "InstallManifest" is used for the type of file that is included in modules to 
# tell ITZ CLI how to install the module.
kind: InstallManifest

# Meta information about this project.
metadata:
  # The namespace for the module. This can be any value right now.
  namespace: IBMTechnologyZone
  # The name of the module. This really should match the name that is displayed
  # to users in software catalogs, etc.
  name: MyModule
  # Any arbitrary labels for the module. Reserved for future use.
  labels:
    "label1": value1

spec:

  # Hooks are not part of the lifecycle of the module but are called at various
  # points during the lifecycle to validate state and lifecycle completeness.
  hooks:
    # Uses the container specified by "image" to get a list of the parameters
    # for the project. This is either a custom container or command specified
    # by the maintainer, or could be a "plugin" that is supported by the
    # ITZ CLI.
    list:
      image: something/parameter-lister:latest
      env:
        - name: MY_PROJECT_NAME
          value: my-base-project
      volumeMounts:
        - mountPath: /workspace
          name: ${HOME}/.itz/cache

    # Similar to list (above), but uses the container to validate the values
    # for the parameters.
    validate:
      image: something/parameter-validator:latest

    # Gets the current state of the project and returns a structure documented at
    get_state:
      image: something/get-stater:latest

  lifecycle:

    # Uses the container specified by image to run any pre-deployment tasks for
    # the project. This could be, for example, generating files in the project
    # based on metadata before actually starting the deployment step.
    pre_deploy:
      image: something/pre-deployer:latest

    # Uses the container specified by image to run the deployment
    deploy:
      image: something/deployer:latest

    # Uses the container specified by image to run post-deployment steps, such
    # as clean-ups, notifications, etc.
    post_deploy:
      image: something/post-deployer:latest
```

## The included Podman/Docker API

In order to read the `img` tag in the module manifest and do something with it, capturing
the output, errors, etc. in an elegant fashion, I implemented a command builder using the
[Builder Patter](https://en.wikipedia.org/wiki/Builder_pattern) and also incorporated the
notion of contexts, which is similar to a [Pipeline](https://en.wikipedia.org/wiki/Component-based_software_engineering)
in that several commands can be strung together (for example, to execute the entire lifecycle
of a module) and be contextually aware.

An example of using the `PodmanCliCommandBuilder` is shown here:

```go
builder := atk.NewPodmanCliCommandBuilder(nil)

actual, err := builder.WithVolume("/home/myuser/workdir").
WithImage("localhost/myimage").
WithEnvvar("MYVAR", "thisismyvalue").
Build()

assert.Nil(t, err)
assert.Equal(t, "/usr/local/bin/podman run --rm -v /home/myuser/workdir:/workspace -e MYVAR=thisismyvalue localhost/myimage", actual)
```

More examples of using the builder can be found in [podmanclibuilder_test.go](test/podmanclibuilder_test.go).

## Developing your own plugin

There are few basic rules for the plugins:

1. Make sure to check the plugin documentation to understand the required STDIN
and/or STDOUT of the plugin.
1. Make sure to use proper UNIX exit codes--use zero for success and non-zero 
for failure or error conditions. For your own trouble-shooting, consider making
your non-zero exit codes mean something--that is, a code of 135 means something
different than a 140.
1. Use STDOUT and STDERR properly. STDOUT is reserved for JSON output sent to
executors, while STDERR is should be used for process debugging or logging
messages that are either displayed to a console or printed to a log file.

Fortunately, there (will be) a container that you can call in your CI/CD
pipeline to validate

## Reference implementations

Reference implementations are in progress.

## FAQ

### Why not just use... \[TravisCI, Drone, Jenkins, kubectl, Airflow, WorkflowXYX...\]

I looked, and looked pretty hard, and even evaluated some of the command line
runners such as that of AWS CodeBuild. Afterall, this is primarily a Day Zero
installer--Day One operations should be handled by GitOps, DevOps, or DevSecOps
pipelines. For a while, even, I used a Jenkins container and tried to basically
implement this within Jenkins.

However, it turned out that building a very lightweight, purpose-built executor
that deferred execution to container-based plugins was the best solution for
both speed of development, lowest impact on modules, backwards compatibility,
and future-proofing. For example, any current Terraform project such as that
currently deployed in TechZone should--for the most part--simply be able to use
the supported plugins and therefore require no additional code other than the
`itz-manifest.yaml` file.

### Why not just use settle on one specific tech (eg., Terraform) and use its built-in goodness?

This didn't seem like a realistic goal, long-term, and likely would drive some
sub-optimal behavior. For example, Ansible is a great choice for deploying and
managing configuration and infrastructure, so providing the ability to use
Terraform or CloudFormation or Bicep for some infrastructure while providing
Ansible or Helm or make for others is inline with both hybrid cloud and
heterogeneous ecosystem approaches.
