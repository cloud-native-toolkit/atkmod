package test

import (
	"bytes"
	cloudevents "github.com/cloudevents/sdk-go/v2"
	"os/exec"
	"reflect"
	"strings"
	"testing"

	atk "github.com/cloud-native-toolkit/atkmod"
	"github.com/stretchr/testify/assert"
)

func TestSupportedApiVersion(t *testing.T) {
	moduleLoader := atk.NewAtkManifestFileLoader()
	module, err := moduleLoader.Load("examples/module3.yml")
	assert.Nil(t, err)
	assert.Equal(t, "itzcli/v1alpha1", module.ApiVersion)
	assert.Equal(t, "InstallManifest", module.Kind)
	assert.True(t, module.IsSupportedKind())
	assert.True(t, module.IsSupportedVersion())
	assert.True(t, module.IsSupported())
}

func TestUnsupportedApiVersion(t *testing.T) {
	moduleLoader := atk.NewAtkManifestFileLoader()
	module, err := moduleLoader.Load("examples/module5.yml")
	assert.Error(t, err)
	assert.Equal(t, "itzcli/v1beta1", module.ApiVersion)
	assert.Equal(t, "InstallManifest", module.Kind)
	assert.True(t, module.IsSupportedKind())
	assert.False(t, module.IsSupportedVersion())
	assert.False(t, module.IsSupported())
}

func TestUnsupportedApiVersionNamespace(t *testing.T) {
	moduleLoader := atk.NewAtkManifestFileLoader()
	module, err := moduleLoader.Load("examples/module6.yml")
	assert.Error(t, err)
	assert.Equal(t, "itzinator/v1alpha1", module.ApiVersion)
	assert.Equal(t, "InstallManifest", module.Kind)
	assert.True(t, module.IsSupportedKind())
	assert.False(t, module.IsSupportedVersion())
	assert.False(t, module.IsSupported())
}

func TestUnsupportedKind(t *testing.T) {
	moduleLoader := atk.NewAtkManifestFileLoader()
	module, err := moduleLoader.Load("examples/module7.yml")
	assert.Error(t, err)
	assert.Equal(t, "itzcli/v1alpha1", module.ApiVersion)
	assert.Equal(t, "NeatoFile", module.Kind)
	assert.False(t, module.IsSupportedKind())
	assert.True(t, module.IsSupportedVersion())
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
	assert.True(t, strings.Contains(buf.String(), "No such file or directory"))
	cmdStr, _ := runner.Build()
	assert.Equal(t, "/bin/ls moo", cmdStr)

	assert.True(t, ctx.IsErrored())
	assert.True(t, len(ctx.Errors) > 0)
	expectedErr := ctx.Errors[0]
	if exiterr, ok := expectedErr.(*exec.ExitError); ok {
		assert.NotEqual(t, 0, exiterr.ExitCode())
	} else {
		assert.Fail(t, "Expected ExitError, got %T", expectedErr)
	}

}

func TestLoadEventData(t *testing.T) {
	type args struct {
		eventS string
	}
	tests := []struct {
		name    string
		args    args
		want    *atk.EventData
		wantErr bool
	}{
		{
			name: "valid event data",
			args: args{
				eventS: `{
  "specversion": "1.0",
  "type": "com.ibm.techzone.itz.hook.list.response",
  "source": "https://github.ibm.com/skol/itz-deployer-plugins/tf-hook-list",
  "subject": "fyre-vm",
  "id": "7208f364-86af-4d18-8fcd-c1f5cd06cdb4",
  "time": "2023-02-13T17:17:48.57Z",
  "datacontenttype": "application/json",
  "data": {
    "variables": [
      {
        "name": "TF_VAR_cloud_provider",
        "default": "fyre"
      },
      {
        "name": "TF_VAR_cloud_type",
        "default": "private"
      },
      {
        "name": "TF_VAR_fyre_api_key",
        "default": ""
      },
      {
        "name": "TF_VAR_fyre_username",
        "default": ""
      }
    ]
  }
}`,
			},
			want: &atk.EventData{
				Variables: []atk.EventDataVarInfo{
					{
						Name:    "TF_VAR_cloud_provider",
						Default: "fyre",
					},
					{
						Name:    "TF_VAR_cloud_type",
						Default: "private",
					},
					{
						Name: "TF_VAR_fyre_api_key",
					},
					{
						Name: "TF_VAR_fyre_username",
					},
				},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event, err := atk.LoadEvent(tt.args.eventS)
			got, err := atk.LoadEventData(event)
			if (err != nil) != tt.wantErr {
				t.Errorf("LoadEventData() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("LoadEventData() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestWriteEvent(t *testing.T) {
	event := cloudevents.NewEvent()
	vars := &atk.EventData{}
	event.SetType(string(atk.ListHookResponseEvent))
	event.SetData(cloudevents.ApplicationJSON, vars)
	outbuff := new(bytes.Buffer)
	err := atk.WriteEvent(&event, outbuff)
	assert.NoError(t, err)
	assert.NotEmpty(t, outbuff.String())
}
