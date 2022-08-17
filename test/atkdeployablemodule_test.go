package test

import (
	"bytes"
	"context"
	"os"
	"testing"

	logger "github.com/sirupsen/logrus"
	logtest "github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
	atk "github.ibm.com/Nathan-Good/atkmod"
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
	handler, exists := deployment.Next()

	handler(runCtx, deployment)

	assert.True(t, exists)
	assert.Equal(t, 1, len(hook.Entries))
	assert.Equal(t, logger.InfoLevel, hook.LastEntry().Level)
	assert.Equal(t, "running command: /usr/local/bin/podman run --rm -v /tmp:/workspace -e MYVAR=thisismyvalue localhost/atk-predeployer", hook.LastEntry().Message)
	assert.Equal(t, "pre deploying...\n", outbuff.String())

}
