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

	module := &atk.AtkModule{
		Specifications: atk.SpecInfo{
			Deploy: *deployImg,
		},
	}

	ctx := context.Background()
	ctx = context.WithValue(ctx, atk.BaseDirectory, "/tmp")
	outbuff := new(bytes.Buffer)
	errbuff := new(bytes.Buffer)

	ctx = context.WithValue(ctx, atk.LoggerContextKey, *log)
	ctx = context.WithValue(ctx, atk.StdOutContextKey, *outbuff)
	ctx = context.WithValue(ctx, atk.StdErrContextKey, *errbuff)

	cfg := &atk.AtkRunCfg{
		Stdout: outbuff,
		Stderr: outbuff,
		Logger: log,
	}

	deployment := atk.NewAtkDeployableModule(ctx, cfg, module)
	err := deployment.Deploy(ctx)

	assert.Nil(t, err)
	assert.Equal(t, 1, len(hook.Entries))
	assert.Equal(t, logger.InfoLevel, hook.LastEntry().Level)
	assert.Equal(t, "running command: /usr/local/bin/podman run --rm -v /tmp:/workspace -e MYVAR=thisismyvalue localhost/atk-predeployer", hook.LastEntry().Message)
	assert.Equal(t, "deploying...\n", outbuff.String())

}