package atkmod

import (
	"bytes"
	"context"
	"fmt"
	"html/template"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"

	logger "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
)

type AtkContextKey string

const (
	LoggerContextKey AtkContextKey = "atk.logger"
	StdOutContextKey AtkContextKey = "atk.stdout"
	StdErrContextKey AtkContextKey = "atk.stderr"
	BaseDirectory    AtkContextKey = "atk.basedir"
)

type EnvVarInfo struct {
	Name  string `json:"name" yaml:"name"`
	Value string `json:"value" yaml:"value"`
}

func (e EnvVarInfo) String() string {
	return fmt.Sprintf("%s=%s", e.Name, e.Value)
}

type ImageInfo struct {
	Image    string       `json:"img" yaml:"img"`
	Commands []string     `json:"cmd" yaml:"cmd"`
	EnvVars  []EnvVarInfo `json:"env" yaml:"env"`
}

type ParamInfo struct {
	List     ImageInfo `json:"list" yaml:"list"`
	Validate ImageInfo `json:"validate" yaml:"validate"`
}

type MetaInfo struct {
	Params ParamInfo `json:"params" yaml:"params"`
}

type SpecInfo struct {
	PreDeploy  ImageInfo `json:"pre_deploy" yaml:"pre_deploy"`
	Deploy     ImageInfo `json:"deploy" yaml:"deploy"`
	PostDeploy ImageInfo `json:"post_deploy" yaml:"post_deploy"`
}

type ModuleInfo struct {
	Id             string   `json:"id" yaml:"id"`
	Name           string   `json:"name" yaml:"name"`
	Version        string   `json:"version" yaml:"version"`
	TemplateUrl    string   `json:"template_url" yaml:"template_url"`
	Facets         []string `json:"facets" yaml:"facets"`
	Dependencies   []string `json:"dependencies" yaml:"dependencies"`
	Meta           MetaInfo `json:"meta" yaml:"meta"`
	Specifications SpecInfo `json:"spec" yaml:"spec"`
}

type CliParts struct {
	Cmd      string
	Image    string
	Flags    []string
	Workdir  string
	Localdir string
	Envvars  []EnvVarInfo
	// TODO: Add command support
	Commands []string
}

type PodmanCliCommandBuilder struct {
	parts CliParts
}

func (b *PodmanCliCommandBuilder) WithImage(imageName string) *PodmanCliCommandBuilder {
	b.parts.Image = imageName
	return b
}

func (b *PodmanCliCommandBuilder) WithVolume(localdir string) *PodmanCliCommandBuilder {
	b.parts.Localdir = localdir
	return b
}

func (b *PodmanCliCommandBuilder) WithEnvvar(name string, value string) *PodmanCliCommandBuilder {
	envar := &EnvVarInfo{
		Name:  name,
		Value: value,
	}
	b.parts.Envvars = append(b.parts.Envvars, *envar)
	return b
}

func (b *PodmanCliCommandBuilder) Build() (string, error) {
	buf := new(bytes.Buffer)
	tmpl, err := template.New("cli").Parse("/usr/local/bin/podman {{.Cmd}} {{range .Flags}}{{.}} {{end}}-v {{.Localdir}}:{{.Workdir}} {{range .Envvars}}-e {{.}} {{end}}{{.Image}}")
	if err == nil {
		tmpl.Execute(buf, b.parts)
		return buf.String(), nil
	}

	return "", err
}

func (b *PodmanCliCommandBuilder) BuildFrom(info ImageInfo) (string, error) {
	b.WithImage(info.Image)
	for _, envvar := range info.EnvVars {
		b.WithEnvvar(envvar.Name, envvar.Value)
	}
	return b.Build()
}

func NewPodmanCliCommandBuilder() *PodmanCliCommandBuilder {
	// TODO: Load from some type of default or configuration
	parts := &CliParts{
		Cmd:     "run",
		Workdir: "/workspace",
		Flags:   []string{"--rm"},
		Envvars: []EnvVarInfo{},
	}
	return &PodmanCliCommandBuilder{
		parts: *parts,
	}
}

type RunContext struct {
	In  io.Reader
	Out io.Writer
	Log logger.Logger
	Err io.Writer
}

type CliModuleRunner struct {
	PodmanCliCommandBuilder
}

// RunImage
func (r *CliModuleRunner) Run(ctx *RunContext, info ImageInfo) error {
	cmdStr, err := r.BuildFrom(info)
	if err != nil {
		return err
	}

	ctx.Log.Infof("running command: %s", cmdStr)
	// TODO: Here we will actually run the command.
	cmdParts := strings.Split(cmdStr, " ")
	runCmd := exec.Command(cmdParts[0], cmdParts[1:]...)
	runCmd.Stdout = ctx.Out
	runCmd.Stderr = ctx.Err

	err = runCmd.Run()

	return err
}

type Status string

const (
	None          Status = "none"
	Invalid       Status = "invalid"
	Initializing  Status = "initializing"
	Configured    Status = "configured"
	Validated     Status = "validated"
	PreDeploying  Status = "predeploying"
	PreDeployed   Status = "predeployed"
	Deploying     Status = "deploying"
	Deployed      Status = "deployed"
	PostDeploying Status = "postdeploying"
	PostDeployed  Status = "postdeployed"
	Done          Status = PostDeployed
	Errored       Status = "errored"
)

var DefaultOrder []Status = []Status{
	Invalid,
	Initializing,
	Configured,
	Validated,
	PreDeploying,
	PreDeployed,
	Deploying,
	Deployed,
	PostDeploying,
	PostDeployed,
	Done,
}

type StateHandler func(ctx *RunContext, notifier Notifier) error

func NoopHandler(ctx *RunContext, notifier Notifier) error {
	notifier.Notify(Invalid)
	return nil
}

type Notifier interface {
	Status() Status
	Notify(Status) error
}

type HandlerCommander interface {
	AddHandler(Status, StateHandler) error
	GetHandlerFor(Status) StateHandler
	Next() (StateHandler, bool)
}

type DeployableModule struct {
	module    *ModuleInfo
	cli       *CliModuleRunner
	handlers  map[Status]StateHandler
	previous  Status
	current   Status
	execOrder []Status
}

func (m *DeployableModule) Status() Status {
	return m.current
}

func (m *DeployableModule) Notify(state Status) error {
	m.previous = m.current
	m.current = state
	return nil
}

func (m *DeployableModule) AddHandler(status Status, handler StateHandler) error {
	if m.handlers[status] == nil {
		m.handlers[status] = handler
		return nil
	} else {
		return fmt.Errorf("handler for state %s already exists", status)
	}
}

func (m *DeployableModule) GetHandlerFor(status Status) StateHandler {
	return m.handlers[status]
}

func (m *DeployableModule) Next() (StateHandler, bool) {
	for idx, status := range m.execOrder {
		if m.current == status {
			return m.GetHandlerFor(m.execOrder[idx+1]), true
		}
	}
	return NoopHandler, false
}

func (m *DeployableModule) preDeploy(ctx *RunContext, notifier Notifier) error {
	notifier.Notify(PreDeploying)
	err := m.cli.Run(ctx, m.module.Specifications.PreDeploy)
	if err != nil {
		notifier.Notify(Errored)
	} else {
		notifier.Notify(PreDeployed)
	}
	return err
}

func (m *DeployableModule) deploy(ctx *RunContext, notifier Notifier) error {
	notifier.Notify(Deploying)
	err := m.cli.Run(ctx, m.module.Specifications.Deploy)
	if err != nil {
		notifier.Notify(Errored)
	} else {
		notifier.Notify(Deployed)
	}
	return err
}

func (m *DeployableModule) postDeploy(ctx *RunContext, notifier Notifier) error {
	notifier.Notify(PostDeploying)
	err := m.cli.Run(ctx, m.module.Specifications.PostDeploy)
	if err != nil {
		notifier.Notify(Errored)
	} else {
		notifier.Notify(PostDeployed)
	}
	return err
}

func (m *DeployableModule) resolveState(ctx *RunContext, notifier Notifier) error {
	// err := m.cli.Run(ctx, m.module.Specifications.PostDeploy)
	// TODO: From this one, we grab the output from the context and
	// use that to notify the state of the current module
	notifier.Notify(Configured)
	return nil
}

func (m *DeployableModule) IsErrored() bool {
	return m.current == Errored
}

func NewDeployableModule(ctx context.Context, runCtx *RunContext, module *ModuleInfo) *DeployableModule {
	builder := NewPodmanCliCommandBuilder()
	cwd := fmt.Sprintf("%s", ctx.Value(BaseDirectory))
	if len(cwd) == 0 {
		cwd, _ = os.Getwd()
	}
	builder = builder.WithVolume(cwd)

	deployment := &DeployableModule{
		module:    module,
		cli:       &CliModuleRunner{*builder},
		execOrder: DefaultOrder,
		current:   Invalid,
		handlers:  make(map[Status]StateHandler),
	}

	// Now configure the handlers for the module deployment
	deployment.AddHandler(PreDeploying, deployment.preDeploy)
	deployment.AddHandler(Deploying, deployment.deploy)
	deployment.AddHandler(PostDeploying, deployment.postDeploy)
	deployment.AddHandler(Initializing, deployment.resolveState)

	return deployment
}

type ModuleLoader interface {
	Load(uri string) (ModuleInfo, error)
}

type ManifestFileLoader struct {
	path string
}

func (l *ManifestFileLoader) Load(uri string) (*ModuleInfo, error) {
	l.path = uri
	logger.Debug("Loading module from manifest file")
	var module *ModuleInfo = &ModuleInfo{}
	yamlFile, err := ioutil.ReadFile(uri)
	if err != nil {
		return nil, err
	}
	err = yaml.Unmarshal(yamlFile, &module)
	return module, err

}

func NewAtkManifestFileLoader() *ManifestFileLoader {
	return &ManifestFileLoader{}
}
