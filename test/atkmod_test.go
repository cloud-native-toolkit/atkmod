package test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	atk "github.ibm.com/Nathan-Good/atkmod"
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
