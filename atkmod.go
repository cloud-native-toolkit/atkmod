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
	GetState   ImageInfo `json:"get_state" yaml:"get_state"`
	PreDeploy  ImageInfo `json:"pre_deploy" yaml:"pre_deploy"`
	Deploy     ImageInfo `json:"deploy" yaml:"deploy"`
	PostDeploy ImageInfo `json:"post_deploy" yaml:"post_deploy"`
}

type ModuleInfo struct {
	Id             string   `json:"id" yaml:"id"`
	Name           string   `json:"name" yaml:"name"`
	Version        string   `json:"version" yaml:"version"`
	TemplateUrl    string   `json:"template_url" yaml:"template_url"`
	Dependencies   []string `json:"dependencies" yaml:"dependencies"`
	Meta           MetaInfo `json:"meta" yaml:"meta"`
	Specifications SpecInfo `json:"spec" yaml:"spec"`
}

type CliParts struct {
	Path     string
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

func (b *PodmanCliCommandBuilder) WithPath(path string) *PodmanCliCommandBuilder {
	b.parts.Path = path
	return b
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
	tmpl, err := template.New("cli").Parse("{{.Path}} {{.Cmd}} {{range .Flags}}{{.}} {{end}}-v {{.Localdir}}:{{.Workdir}} {{range .Envvars}}-e {{.}} {{end}}{{.Image}}")
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

func NewPodmanCliCommandBuilder(cli *CliParts) *PodmanCliCommandBuilder {
	defaults := cli
	if defaults == nil {
		defaults = &CliParts{}
	}
	defaultFlags := []string{"--rm"}
	parts := &CliParts{
		Path:    Iif(defaults.Path, "/usr/local/bin/podman"),
		Cmd:     Iif(defaults.Cmd, "run"),
		Workdir: Iif(defaults.Workdir, "/workspace"),
		Flags:   append(defaults.Flags, defaultFlags...),
		Envvars: defaults.Envvars,
	}
	return &PodmanCliCommandBuilder{
		parts: *parts,
	}
}

func Iif(value string, orValue string) string {
	if len(strings.TrimSpace(value)) == 0 {
		return orValue
	}
	return value
}

type RunContext struct {
	In     io.Reader
	Out    io.Writer
	Log    logger.Logger
	Err    io.Writer
	Errors []error
}

// AddError adds an error to the context
func (c *RunContext) AddError(err error) {
	if c.Errors == nil {
		c.Errors = make([]error, 0)
	}
	c.Errors = append(c.Errors, err)
}

// IsErrored returns true if there are errors in the context
func (c *RunContext) IsErrored() bool {
	return len(c.Errors) > 0
}

type CliModuleRunner struct {
	PodmanCliCommandBuilder
}

func (r *CliModuleRunner) runCmd(ctx *RunContext, cmd string) error {
	ctx.Log.Infof("running command: %s", cmd)
	cmdParts := strings.Split(cmd, " ")
	runCmd := exec.Command(cmdParts[0], cmdParts[1:]...)
	runCmd.Stdout = ctx.Out
	runCmd.Stderr = ctx.Err
	err := runCmd.Run()
	if err != nil {
		ctx.AddError(err)
	}
	return err
}

// RunImage
func (r *CliModuleRunner) RunImage(ctx *RunContext, info ImageInfo) error {
	cmdStr, err := r.BuildFrom(info)
	if err != nil {
		ctx.AddError(err)
		return err
	}

	return r.runCmd(ctx, cmdStr)
}

// Run
func (r *CliModuleRunner) Run(ctx *RunContext) error {
	cmdStr, err := r.Build()
	if err != nil {
		ctx.AddError(err)
		return err
	}

	return r.runCmd(ctx, cmdStr)
}

type State string

const (
	None          State = "none"
	Invalid       State = "invalid"
	Initializing  State = "initializing"
	Configured    State = "configured"
	Validated     State = "validated"
	PreDeploying  State = "predeploying"
	PreDeployed   State = "predeployed"
	Deploying     State = "deploying"
	Deployed      State = "deployed"
	PostDeploying State = "postdeploying"
	PostDeployed  State = "postdeployed"
	Done          State = PostDeployed
	Errored       State = "errored"
)

var DefaultOrder []State = []State{
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

// StateCmd is an implementation of a Command pattern
type StateCmd func(ctx *RunContext, notifier Notifier) error

// NoopHandler is an implementation of the Null Object pattern.
// It does nothing except to insure we don't return a nil.
func NoopHandler(ctx *RunContext, notifier Notifier) error {
	notifier.Notify(Invalid)
	return nil
}

type Notifier interface {
	State() State
	Notify(State) error
	NotifyErr(State, error)
}

type StateCmder interface {
	AddCmd(State, StateCmd) error
	GetCmdFor(State) StateCmd
}

type CmdItr interface {
	Next() (StateCmd, bool)
}

type DeployableModule struct {
	module    *ModuleInfo
	cli       *CliModuleRunner
	runCtx    RunContext
	cmds      map[State]StateCmd
	previous  State
	current   State
	execOrder []State
}

func (m *DeployableModule) State() State {
	return m.current
}

func (m *DeployableModule) Notify(state State) error {
	m.previous = m.current
	m.current = state
	return nil
}

func (m *DeployableModule) NotifyErr(state State, err error) {
	m.runCtx.AddError(err)
	m.previous = m.current
	m.current = state
}

func (m *DeployableModule) AddCmd(status State, handler StateCmd) error {
	if m.cmds[status] == nil {
		m.cmds[status] = handler
		return nil
	} else {
		return fmt.Errorf("handler for state %s already exists", status)
	}
}

func (m *DeployableModule) GetCmdFor(status State) StateCmd {
	return m.cmds[status]
}

func (m *DeployableModule) Next() (StateCmd, bool) {
	for idx, state := range m.execOrder {
		if m.current == state {
			return m.GetCmdFor(m.execOrder[idx+1]), true
		}
	}
	return NoopHandler, false
}

func (m *DeployableModule) preDeploy(ctx *RunContext, notifier Notifier) error {
	notifier.Notify(PreDeploying)
	err := m.cli.RunImage(ctx, m.module.Specifications.PreDeploy)
	if err != nil {
		notifier.Notify(Errored)
	} else {
		notifier.Notify(PreDeployed)
	}
	return err
}

func (m *DeployableModule) deploy(ctx *RunContext, notifier Notifier) error {
	notifier.Notify(Deploying)
	err := m.cli.RunImage(ctx, m.module.Specifications.Deploy)
	if err != nil {
		notifier.Notify(Errored)
	} else {
		notifier.Notify(Deployed)
	}
	return err
}

func (m *DeployableModule) postDeploy(ctx *RunContext, notifier Notifier) error {
	notifier.Notify(PostDeploying)
	err := m.cli.RunImage(ctx, m.module.Specifications.PostDeploy)
	if err != nil {
		notifier.Notify(Errored)
	} else {
		notifier.Notify(PostDeployed)
	}
	return err
}

func (m *DeployableModule) resolveState(ctx *RunContext, notifier Notifier) error {
	// err := m.cli.RunImage(ctx, m.module.Specifications.PostDeploy)
	// TODO: From this one, we grab the output from the context and
	// use that to notify the state of the current module
	notifier.Notify(Configured)
	return nil
}

func (m *DeployableModule) IsErrored() bool {
	return m.current == Errored
}

func NewDeployableModule(ctx context.Context, runCtx *RunContext, module *ModuleInfo) *DeployableModule {
	builder := NewPodmanCliCommandBuilder(nil)
	cwd := fmt.Sprintf("%s", ctx.Value(BaseDirectory))
	if len(cwd) == 0 {
		cwd, _ = os.Getwd()
	}
	builder = builder.WithVolume(cwd)

	deployment := &DeployableModule{
		module:    module,
		cli:       &CliModuleRunner{*builder},
		runCtx:    *runCtx,
		execOrder: DefaultOrder,
		current:   Invalid,
		cmds:      make(map[State]StateCmd),
	}

	// Now configure the cmds for the module deployment
	deployment.AddCmd(PreDeploying, deployment.preDeploy)
	deployment.AddCmd(Deploying, deployment.deploy)
	deployment.AddCmd(PostDeploying, deployment.postDeploy)
	deployment.AddCmd(Initializing, deployment.resolveState)

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
