package test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	atk "github.ibm.com/skol/atkmod"
)

func TestBuildRun(t *testing.T) {

	builder := atk.NewPodmanCliCommandBuilder(nil)

	actual, err := builder.WithVolume("/home/myuser/workdir").
		WithImage("localhost/myimage").
		WithEnvvar("MYVAR", "thisismyvalue").
		Build()

	assert.Nil(t, err)
	assert.Equal(t, "/usr/local/bin/podman run -v /home/myuser/workdir:/workspace -e MYVAR=thisismyvalue localhost/myimage", actual)

}

func TestBuildFrom(t *testing.T) {

	builder := atk.NewPodmanCliCommandBuilder(nil)

	imageInfo := &atk.ImageInfo{
		Image: "localhost/myimage",
		EnvVars: []atk.EnvVarInfo{
			{Name: "MYVAR", Value: "thisismyvalue"},
		},
	}

	actual, err := builder.WithVolume("/home/myuser/workdir").
		BuildFrom(*imageInfo)

	assert.Nil(t, err)
	assert.Equal(t, "/usr/local/bin/podman run -v /home/myuser/workdir:/workspace -e MYVAR=thisismyvalue localhost/myimage", actual)

}

func TestProvideOverrides(t *testing.T) {

	cli := &atk.CliParts{
		Path:  "/usr/bin/docker",
		Cmd:   "build",
		Flags: []string{"-d", "--rm"},
	}

	builder := atk.NewPodmanCliCommandBuilder(cli)

	imageInfo := &atk.ImageInfo{
		Image: "localhost/myimage",
		EnvVars: []atk.EnvVarInfo{
			{Name: "MYVAR", Value: "thisismyvalue"},
		},
	}

	actual, err := builder.WithVolume("/home/myuser/workdir").
		BuildFrom(*imageInfo)

	assert.Nil(t, err)
	assert.Equal(t, "/usr/bin/docker build -d --rm -v /home/myuser/workdir:/workspace -e MYVAR=thisismyvalue localhost/myimage", actual)
}

func TestPsCommandOnly(t *testing.T) {
	cfg := &atk.CliParts{
		Path: "",
		Cmd:  `ps --format "{{.Image}}"`,
	}

	bldr := atk.NewPodmanCliCommandBuilder(cfg)
	actual, err := bldr.Build()
	assert.Nil(t, err)
	assert.Equal(t, "/usr/local/bin/podman ps --format \"{{.Image}}\"", actual)
}
