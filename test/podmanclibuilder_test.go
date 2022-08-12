package test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	atk "github.ibm.com/Nathan-Good/atkmod"
)

func TestBuildRun(t *testing.T) {

	builder := atk.NewPodmanCliCommandBuilder()

	actual, err := builder.WithVolume("/home/myuser/workdir").
		WithImage("localhost/myimage").
		WithEnvvar("MYVAR", "thisismyvalue").
		Build()

	assert.Nil(t, err)
	assert.Equal(t, "/usr/local/bin/podman run --rm -v /home/myuser/workdir:/workspace -e MYVAR=thisismyvalue localhost/myimage", actual)

}

func TestBuildFrom(t *testing.T) {

	builder := atk.NewPodmanCliCommandBuilder()

	imageInfo := &atk.ImageInfo{
		Image: "localhost/myimage",
		EnvVars: []atk.EnvVarInfo{
			{Name: "MYVAR", Value: "thisismyvalue"},
		},
	}

	actual, err := builder.WithVolume("/home/myuser/workdir").
		BuildFrom(*imageInfo)

	assert.Nil(t, err)
	assert.Equal(t, "/usr/local/bin/podman run --rm -v /home/myuser/workdir:/workspace -e MYVAR=thisismyvalue localhost/myimage", actual)

}
