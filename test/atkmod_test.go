package test

import (
	"bytes"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
	atk "github.ibm.com/skol/atkmod"
)

func TestCreateFromFile(t *testing.T) {
	moduleLoader := atk.NewAtkManifestFileLoader()
	module, err := moduleLoader.Load("examples/module1.yml")

	assert.Nil(t, err)
	assert.Equal(t, "my-base-project", module.Id)
	assert.Equal(t, "My Base Project", module.Name)
	assert.Equal(t, "0.1", module.Version)
	assert.Equal(t, "https://github.com/someorg/someproject", module.TemplateUrl)
	assert.Equal(t, "None", module.Dependencies[0])

	assert.Equal(t, "something/parameter-lister:latest", module.Meta.Params.List.Image)
	assert.Equal(t, "echo \"Running list\"", module.Meta.Params.List.Commands[0])
	assert.Equal(t, "MY_PROJECT_NAME", module.Meta.Params.List.EnvVars[0].Name)
	assert.Equal(t, "my-base-project", module.Meta.Params.List.EnvVars[0].Value)

	assert.Equal(t, "something/parameter-validator:latest", module.Meta.Params.Validate.Image)
	assert.Equal(t, "echo \"Running validate\"", module.Meta.Params.Validate.Commands[0])

	assert.Equal(t, "something/get-stater:latest", module.Specifications.GetState.Image)

	assert.Equal(t, "something/pre-deployer:latest", module.Specifications.PreDeploy.Image)
	assert.Equal(t, "echo \"Running pre-deploy\"", module.Specifications.PreDeploy.Commands[0])

	assert.Equal(t, "something/deployer:latest", module.Specifications.Deploy.Image)
	assert.Equal(t, "echo \"Running deploy\"", module.Specifications.Deploy.Commands[0])

	assert.Equal(t, "something/post-deployer:latest", module.Specifications.PostDeploy.Image)
	assert.Equal(t, "echo \"Running post-deploy\"", module.Specifications.PostDeploy.Commands[0])
}

func TestOutStringFromContext(t *testing.T) {
	buf := new(bytes.Buffer)
	ctx := &atk.RunContext{
		Out: buf,
	}

	ctx.Out.Write([]byte("this is a string that I am writing to the context"))
	assert.Equal(t, "this is a string that I am writing to the context", buf.String())
}

func TestLastErrCode(t *testing.T) {
	defaults := &atk.CliParts{
		Path: `/bin/ls`,
		Cmd:  `moo`,
	}

	buf := new(bytes.Buffer)
	ctx := &atk.RunContext{
		Err: buf,
	}
	assert.Equal(t, 0, ctx.LastErrCode, "Should be zero after fresh creation.")

	cli := atk.NewPodmanCliCommandBuilder(defaults)
	runner := atk.CliModuleRunner{PodmanCliCommandBuilder: *cli}
	runner.Run(ctx)
	assert.Equal(t, "ls: moo: No such file or directory\n", buf.String())
	cmdStr, _ := runner.Build()
	assert.Equal(t, "/bin/ls moo", cmdStr)

	assert.True(t, ctx.IsErrored())
	assert.True(t, len(ctx.Errors) > 0)
	expectedErr := ctx.Errors[0]
	if exiterr, ok := expectedErr.(*exec.ExitError); ok {
		assert.Equal(t, 1, exiterr.ExitCode())
	} else {
		assert.Fail(t, "Expected ExitError, got %T", expectedErr)
	}

}
