package test

import (
	"bytes"
	"context"
	"os"
	"strings"
	"testing"

	atk "github.com/cloud-native-toolkit/atkmod"
	logger "github.com/sirupsen/logrus"
	logtest "github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
)

func TestRunHappyPathFullDeployment(t *testing.T) {
	loader := atk.NewAtkManifestFileLoader()
	manifest, err := loader.Load("examples/module3.yml")
	assert.NoError(t, err)
	outbuff := new(bytes.Buffer)
	errbuff := new(bytes.Buffer)

	// TODO: Move this to a private func
	// This is only required for unit testing, else normal logrus logger works.
	log, _ := logtest.NewNullLogger()
	log.SetFormatter(&logger.TextFormatter{})
	log.SetOutput(os.Stdout)
	log.SetLevel(logger.DebugLevel)

	runCtx := &atk.RunContext{
		Context: context.Background(),
		Out:     outbuff,
		Err:     errbuff,
		Log:     *log,
	}
	module := atk.NewDeployableModule(runCtx, manifest)

	i := 0
	var step atk.StateCmd
	for next, hasNext := module.Itr(); hasNext; i++ {
		step, hasNext = next()
		step(runCtx, module)
		log.Infof("Step %d; running stage %s with output: %s", i, module.State(), outbuff.String())
	}

	assert.False(t, module.IsErrored())
	assert.Equal(t, module.State(), atk.Done)
}

func TestRunDeploymentBadCommends(t *testing.T) {
	loader := atk.NewAtkManifestFileLoader()
	manifest, err := loader.Load("examples/module4.yml")
	assert.NoError(t, err)
	outbuff := new(bytes.Buffer)
	errbuff := new(bytes.Buffer)

	// TODO: Move this to a private func
	// This is only required for unit testing, else normal logrus logger works.
	log, _ := logtest.NewNullLogger()
	log.SetFormatter(&logger.TextFormatter{})
	log.SetOutput(os.Stdout)
	log.SetLevel(logger.DebugLevel)

	runCtx := &atk.RunContext{
		Context: context.Background(),
		Out:     outbuff,
		Err:     errbuff,
		Log:     *log,
	}
	module := atk.NewDeployableModule(runCtx, manifest)

	i := 0
	var step atk.StateCmd
	for next, hasNext := module.Itr(); hasNext; i++ {
		step, hasNext = next()
		err = step(runCtx, module)
		if err != nil {
			log.Errorf("Step %d; running stage %s with error: %s", i, module.State(), err.Error())
			assert.Equal(t, "command is not yet supported", err.Error())
		} else {
			log.Infof("Step %d; running stage %s with output: %s", i, module.State(), outbuff.String())
		}
	}

	assert.True(t, module.IsErrored())
	assert.Equal(t, "", errbuff.String())
	assert.Equal(t, module.State(), atk.Errored)
}

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
		Volumes: []atk.VolumeInfo{{
			MountPath: "/workspace",
			Name:      "/tmp",
		},
		},
	}

	module := &atk.ModuleInfo{
		Specifications: atk.SpecInfo{
			Hooks: atk.HookInfo{},
			Lifecycle: atk.LifecycleInfo{
				PreDeploy: *deployImg,
			},
		},
	}

	outbuff := new(bytes.Buffer)
	errbuff := new(bytes.Buffer)

	runCtx := &atk.RunContext{
		Context: context.Background(),
		Out:     outbuff,
		Err:     errbuff,
		Log:     *log,
	}

	deployment := atk.NewDeployableModule(runCtx, module)
	// For the test purposes, let us just start out with this ready to pre-deploy
	deployment.Notify(atk.PreDeploying)
	// Gets the correct command for the current state
	nextStep, exists := deployment.Itr()
	// Now runs the command
	cmd, exists := nextStep()
	cmd(runCtx, deployment)

	assert.True(t, exists)
	assert.Equal(t, 1, len(hook.Entries))
	assert.Equal(t, logger.InfoLevel, hook.LastEntry().Level)
	assert.Equal(t, "running command: /usr/local/bin/podman run -v /tmp:/workspace -e MYVAR=thisismyvalue localhost/atk-predeployer", hook.LastEntry().Message)
	assert.False(t, runCtx.IsErrored())
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
		Volumes: []atk.VolumeInfo{{
			MountPath: "/workspace",
			Name:      "/tmp",
		},
		},
	}

	module := &atk.ModuleInfo{
		Specifications: atk.SpecInfo{
			Hooks: atk.HookInfo{},
			Lifecycle: atk.LifecycleInfo{
				PreDeploy: *deployImg,
			},
		},
	}

	outbuff := new(bytes.Buffer)
	errbuff := new(bytes.Buffer)

	runCtx := &atk.RunContext{
		Context: context.Background(),
		Out:     outbuff,
		Err:     errbuff,
		Log:     *log,
	}

	deployment := atk.NewDeployableModule(runCtx, module)
	// For the test purposes, let us just start out with this ready to pre-deploy
	deployment.Notify(atk.PreDeploying)
	// Gets the correct command for the current state
	next, exists := deployment.Itr()
	// Now runs the command
	cmd, exists := next()
	cmd(runCtx, deployment)

	//assert.True(t, exists)
	//assert.Equal(t, 1, len(hook.Entries))
	//
	//cmd, exists = next()
	//cmd(runCtx, deployment)

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
		Volumes: []atk.VolumeInfo{{
			MountPath: "/workspace",
			Name:      "/tmp",
		},
		},
	}

	module := &atk.ModuleInfo{
		Specifications: atk.SpecInfo{
			Hooks: atk.HookInfo{},
			Lifecycle: atk.LifecycleInfo{
				PreDeploy: *deployImg,
			},
		},
	}

	outbuff := new(bytes.Buffer)
	errbuff := new(bytes.Buffer)

	runCtx := &atk.RunContext{
		Context: context.Background(),
		Out:     outbuff,
		Err:     errbuff,
		Log:     *log,
	}

	deployment := atk.NewDeployableModule(runCtx, module)
	// For the test purposes, let us just start out with this ready to pre-deploy
	deployment.Notify(atk.PreDeploying)
	// Gets the correct command for the current state
	next, exists := deployment.Itr()
	cmd, exists := next()
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
