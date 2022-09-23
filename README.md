# atkmod: A go library API for module manifests

[![Build Status](https://travis.ibm.com/skol/atkmod.svg?token=wGYsX6PCXyDddvgpBC56&branch=main)](https://travis.ibm.com/skol/atkmod)

> Note: this proof of concept is not dead, but as of Sept 21, 2022 is shelved while some other work.
> That being said, the module is in use by the `atk` [CLI](https://github.ibm.com/skol/atkcli), which
> is also a proof of concept. This module includes some code for dealing with the Podman/Docker
> command line and output that is unit tested, and it was better to re-use this than copy and paste it
> or include it in the CLI outside this module.

The purpose of this project was originally to demonstrate a proof of concept for defining a "module file" that would
be basically a descriptor used in a module's repository to provide a mechanism for dealing with the module
using a Podman-based, container plugin architecture.

In other words, this was intended to prove an approach to answer the questions: 
* *What if I didn't need to care how I got the parameters, or how I even deployed the module?* 
* *What if I left that up to plugins (in the form of containers that emit documented output)?*

## The module manifest file

Examples of the module manifest file are best viewed in the *test/examples* directory, because
those are the files that are run against unit tests and therefore verified against actual code.
But, for convenience, an example is shown here:

```yaml
id: my-base-project
# This is the name of the project
name: My Base Project
# The version of this file spec
version: 0.1
# The URL to the .git repo that is the template for this project.
template_url: https://github.com/someorg/someproject
# The module dependencies, or "None" if there are no dependencies.
dependencies:
  - None

# Meta information about this project.
meta:
  params:
    # Uses the container specified by "img" to get a list of the parameters
    # for the project. This is either a custom container or command specified
    # by the maintainer, or could be a "plugin" that is supported by the
    # ATK team.
    #
    # See https://TODO for documentation on the expected output
    list:
      img: something/parameter-lister:latest
      cmd:
        - echo "Running list"
      env:
        - name: MY_PROJECT_NAME
          value: my-base-project

    # Similar to list (above), but uses the container to validate the values
    # for the parameters.
    # See https://TODO for documentation on the input
    validate:
      img: something/parameter-validator:latest
      cmd:
        - echo "Running validate"

spec:
  # Gets the current state of the project and returns a structure documented at
  # https://TODO
  get_state:
    img: something/get-stater:latest

  # Uses the container specified by img to run any pre-deployment tasks for
  # the project. This could be, for example, generating files in the project
  # based on metadata before actually starting the deployment step.
  pre_deploy:
    img: something/pre-deployer:latest
    cmd:
      - echo "Running pre-deploy"

  # Uses the container specified by img to run the deployment
  deploy:
    img: something/deployer:latest
    cmd:
      - echo "Running deploy"

  # Uses the container specified by img to run post-deployment steps, such
  # as clean-ups, notifications, etc.
  post_deploy:
    img: something/post-deployer:latest
    cmd:
      - echo "Running post-deploy"

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
