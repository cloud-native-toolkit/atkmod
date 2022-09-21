package test

import (
	"bytes"
	"context"
	"os"
	"strings"
	"testing"

	logger "github.com/sirupsen/logrus"
	logtest "github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
	atk "github.ibm.com/skol/atkmod"
)

func TestRunDeployment(t *testing.T) {

	log, hook := logtest.NewNullLogger()
	log.SetFormatter(&logger.TextFormatter{})
	log.SetOutput(os.Stdout)
	log.SetLevel(logger.DebugLevel)

	deployImg := &atk.ImageInfo{
		Image: "localhost/atk-predeployer",
		EnvVars: []atk.EnvVarInfo{
			{Name: "MYVAR", Value: "thisismyvalue"},
		},
	}

	module := &atk.ModuleInfo{
		Specifications: atk.SpecInfo{
			PreDeploy: *deployImg,
		},
	}

	ctx := context.Background()
	ctx = context.WithValue(ctx, atk.BaseDirectory, "/tmp")
	outbuff := new(bytes.Buffer)
	errbuff := new(bytes.Buffer)

	runCtx := &atk.RunContext{
		Out: outbuff,
		Err: errbuff,
		Log: *log,
	}

	deployment := atk.NewDeployableModule(ctx, runCtx, module)
	// For the test purposes, let us just start out with this ready to pre-deploy
	deployment.Notify(atk.Validated)
	// Gets the correct command for the current state
	cmd, exists := deployment.Next()
	// Now runs the command
	cmd(runCtx, deployment)

	assert.True(t, exists)
	assert.Equal(t, 1, len(hook.Entries))
	assert.Equal(t, logger.InfoLevel, hook.LastEntry().Level)
	assert.Equal(t, "running command: /usr/local/bin/podman run -v /tmp:/workspace -e MYVAR=thisismyvalue localhost/atk-predeployer", hook.LastEntry().Message)
	assert.Equal(t, "pre deploying...\n", outbuff.String())

}

func TestContainerWithErr(t *testing.T) {

	log, hook := logtest.NewNullLogger()
	log.SetFormatter(&logger.TextFormatter{})
	log.SetOutput(os.Stdout)
	log.SetLevel(logger.DebugLevel)

	deployImg := &atk.ImageInfo{
		Image: "localhost/atk-errer",
		EnvVars: []atk.EnvVarInfo{
			{Name: "MYVAR", Value: "thisismyvalue"},
		},
	}

	module := &atk.ModuleInfo{
		Specifications: atk.SpecInfo{
			PreDeploy: *deployImg,
		},
	}

	ctx := context.Background()
	ctx = context.WithValue(ctx, atk.BaseDirectory, "/tmp")
	outbuff := new(bytes.Buffer)
	errbuff := new(bytes.Buffer)

	runCtx := &atk.RunContext{
		Out: outbuff,
		Err: errbuff,
		Log: *log,
	}

	deployment := atk.NewDeployableModule(ctx, runCtx, module)
	// For the test purposes, let us just start out with this ready to pre-deploy
	deployment.Notify(atk.Validated)
	// Gets the correct command for the current state
	cmd, exists := deployment.Next()
	// Now runs the command
	cmd(runCtx, deployment)

	assert.True(t, exists)
	assert.Equal(t, 1, len(hook.Entries))
	assert.Equal(t, logger.InfoLevel, hook.LastEntry().Level)
	assert.Equal(t, "running command: /usr/local/bin/podman run -v /tmp:/workspace -e MYVAR=thisismyvalue localhost/atk-errer", hook.LastEntry().Message)
	assert.Equal(t, "", outbuff.String())
	assert.Equal(t, "sh: nowhereisacommandthatdoesnotexist: not found\n", errbuff.String())
	assert.True(t, runCtx.IsErrored())
	assert.Equal(t, 1, len(runCtx.Errors))

}

func TestNonExistImage(t *testing.T) {

	log, hook := logtest.NewNullLogger()
	log.SetFormatter(&logger.TextFormatter{})
	log.SetOutput(os.Stdout)
	log.SetLevel(logger.DebugLevel)

	deployImg := &atk.ImageInfo{
		Image: "localhost/nowhereisanimagethatdoesnotexist",
	}

	module := &atk.ModuleInfo{
		Specifications: atk.SpecInfo{
			PreDeploy: *deployImg,
		},
	}

	ctx := context.Background()
	ctx = context.WithValue(ctx, atk.BaseDirectory, "/tmp")
	outbuff := new(bytes.Buffer)
	errbuff := new(bytes.Buffer)

	runCtx := &atk.RunContext{
		Out: outbuff,
		Err: errbuff,
		Log: *log,
	}

	deployment := atk.NewDeployableModule(ctx, runCtx, module)
	// For the test purposes, let us just start out with this ready to pre-deploy
	deployment.Notify(atk.Validated)
	// Gets the correct command for the current state
	cmd, exists := deployment.Next()
	// Now runs the command
	cmd(runCtx, deployment)

	assert.True(t, exists)
	assert.Equal(t, 1, len(hook.Entries))
	assert.Equal(t, logger.InfoLevel, hook.LastEntry().Level)
	assert.Equal(t, "running command: /usr/local/bin/podman run -v /tmp:/workspace localhost/nowhereisanimagethatdoesnotexist", hook.LastEntry().Message)
	assert.Equal(t, "", outbuff.String())
	assert.True(t, strings.HasPrefix(errbuff.String(), "Trying to pull localhost/nowhereisanimagethatdoesnotexist:latest...\nError: initializing source docker://localhost/nowhereisanimagethatdoesnotexist:latest"))
	assert.True(t, runCtx.IsErrored())
	assert.Equal(t, 1, len(runCtx.Errors))
}
