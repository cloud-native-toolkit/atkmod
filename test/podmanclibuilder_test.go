package test

import (
	"fmt"
	"os"
	"testing"

	atk "github.com/cloud-native-toolkit/atkmod"
	"github.com/stretchr/testify/assert"
)

var testPodmanPath = os.Getenv("ITZ_PODMAN_PATH")

func TestBuildRun(t *testing.T) {

	builder := atk.NewPodmanCliCommandBuilder(nil)

	actual, err := builder.WithWorkspace("/home/myuser/workdir").
		WithImage("myimage").
		WithEnvvar("MYVAR", "thisismyvalue").
		Build()

	assert.Nil(t, err)
	assert.Equal(t, fmt.Sprintf("%s run -v /home/myuser/workdir:/workspace -e MYVAR=thisismyvalue myimage", testPodmanPath), actual)

}

func TestBuildRunWithVolumes(t *testing.T) {
	builder := atk.NewPodmanCliCommandBuilder(nil)
	actual, err := builder.
		WithImage("myimage").
		WithEnvvar("MYVAR", "thisismyvalue").
		WithVolume("/tmp/data", "/var/app/db").
		Build()

	assert.Nil(t, err)
	assert.Equal(t, fmt.Sprintf("%s run -v /tmp/data:/var/app/db -e MYVAR=thisismyvalue myimage", testPodmanPath), actual)

}

func TestBuildRunWithVolumeOpts(t *testing.T) {
	builder := atk.NewPodmanCliCommandBuilder(nil)
	actual, err := builder.
		WithImage("myimage").
		WithEnvvar("MYVAR", "thisismyvalue").
		WithVolumeOpt("/tmp/data", "/var/app/db", "Z").
		Build()

	assert.Nil(t, err)
	assert.Equal(t, fmt.Sprintf("%s run -v /tmp/data:/var/app/db:Z -e MYVAR=thisismyvalue myimage", testPodmanPath), actual)
}

func TestBuildRunWithPorts(t *testing.T) {
	builder := atk.NewPodmanCliCommandBuilder(nil)
	actual, err := builder.
		WithImage("myimage").
		WithPort("80", "8080").
		Build()

	assert.Nil(t, err)
	assert.Equal(t, fmt.Sprintf("%s run -p 80:8080 myimage", testPodmanPath), actual)

}

func TestBuildRunWithUidMap(t *testing.T) {
	builder := atk.NewPodmanCliCommandBuilder(nil)
	actual, err := builder.
		WithImage("myimage").
		WithUserMap(0, 1000, 1).
		WithUserMap(1, 0, 1000).
		Build()

	assert.Nil(t, err)
	assert.Equal(t, fmt.Sprintf("%s run --uidmap 1000:0:1 --uidmap 0:1:1000 myimage", testPodmanPath), actual)
}

func TestBuildFrom(t *testing.T) {

	builder := atk.NewPodmanCliCommandBuilder(nil)

	imageInfo := &atk.ImageInfo{
		Image: "myimage",
		EnvVars: []atk.EnvVarInfo{
			{Name: "MYVAR", Value: "thisismyvalue"},
		},
	}

	actual, err := builder.WithWorkspace("/home/myuser/workdir").
		BuildFrom(*imageInfo)

	assert.Nil(t, err)
	assert.Equal(t, fmt.Sprintf("%s run -v /home/myuser/workdir:/workspace -e MYVAR=thisismyvalue myimage", testPodmanPath), actual)

}

func TestProvideOverrides(t *testing.T) {

	cli := &atk.CliParts{
		Path:  "/usr/bin/docker",
		Cmd:   "build",
		Flags: []string{"-d", "--rm"},
	}

	builder := atk.NewPodmanCliCommandBuilder(cli)

	imageInfo := &atk.ImageInfo{
		Image: "myimage",
		EnvVars: []atk.EnvVarInfo{
			{Name: "MYVAR", Value: "thisismyvalue"},
		},
	}

	actual, err := builder.WithWorkspace("/home/myuser/workdir").
		BuildFrom(*imageInfo)

	assert.Nil(t, err)
	assert.Equal(t, "/usr/bin/docker build -d --rm -v /home/myuser/workdir:/workspace -e MYVAR=thisismyvalue myimage", actual)
}

func TestPsCommandOnly(t *testing.T) {
	cfg := &atk.CliParts{
		Path: "",
		Cmd:  `ps --format "{{.Image}}"`,
	}

	bldr := atk.NewPodmanCliCommandBuilder(cfg)
	actual, err := bldr.Build()
	assert.Nil(t, err)
	assert.Equal(t, fmt.Sprintf("%s ps --format \"{{.Image}}\"", "/usr/local/bin/podman"), actual)
}
