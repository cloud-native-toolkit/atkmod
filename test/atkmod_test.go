package test

import (
	"bytes"
	"os/exec"
	"testing"

	atk "github.com/cloud-native-toolkit/atkmod"
	"github.com/stretchr/testify/assert"
)

func TestUnsupportedApiVersion(t *testing.T) {
	moduleLoader := atk.NewAtkManifestFileLoader()
	module, err := moduleLoader.Load("examples/module3.yml")
	assert.Nil(t, err)
	assert.Equal(t, "itzcli/v1beta1", module.ApiVersion)
	assert.Equal(t, "InstallManifest", module.Kind)
	assert.True(t, module.IsSupportedKind())
	assert.False(t, module.IsSupportedVersion())
	assert.False(t, module.IsSupported())
}

func TestCreateFromFile(t *testing.T) {
	moduleLoader := atk.NewAtkManifestFileLoader()
	module, err := moduleLoader.Load("examples/module1.yml")

	assert.Nil(t, err)
	assert.Equal(t, "itzcli/v1alpha1", module.ApiVersion)
	assert.Equal(t, "InstallManifest", module.Kind)
	assert.True(t, module.IsSupported())

	assert.Equal(t, "something/parameter-lister:latest", module.Specifications.Hooks.List.Image)
	// TODO: Add this test back in when commands are supported.
	//assert.Equal(t, "echo \"Running list\"", module.Specifications.Hooks.List.Command[0])
	assert.Equal(t, "MY_PROJECT_NAME", module.Specifications.Hooks.List.EnvVars[0].Name)
	assert.Equal(t, "my-base-project", module.Specifications.Hooks.List.EnvVars[0].Value)

	assert.Equal(t, "something/parameter-validator:latest", module.Specifications.Hooks.Validate.Image)
	//assert.Equal(t, "echo \"Running validate\"", module.Specifications.Hooks.Validate.Command[0])

	assert.Equal(t, "something/get-stater:latest", module.Specifications.Hooks.GetState.Image)

	assert.Equal(t, "something/pre-deployer:latest", module.Specifications.Lifecycle.PreDeploy.Image)
	//assert.Equal(t, "echo \"Running pre-deploy\"", module.Specifications.Lifecycle.PreDeploy.Command[0])

	assert.Equal(t, "something/deployer:latest", module.Specifications.Lifecycle.Deploy.Image)
	//assert.Equal(t, "echo \"Running deploy\"", module.Specifications.Lifecycle.Deploy.Command[0])

	assert.Equal(t, "something/post-deployer:latest", module.Specifications.Lifecycle.PostDeploy.Image)
	//assert.Equal(t, "echo \"Running post-deploy\"", module.Specifications.Lifecycle.PostDeploy.Command[0])
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
