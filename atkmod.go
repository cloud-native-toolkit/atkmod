package atkmod

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"
	"text/template"

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

// CliParts represents the parts of the entire podman command line.
type CliParts struct {
	Path             string
	Cmd              string
	Image            string
	Flags            []string
	Workdir          string
	VolumeMaps       []string
	DefaultVolumeOpt string
	Ports            map[string]string
	UidMaps          []string
	Envvars          []EnvVarInfo
	// TODO: Add command support that will be used instead of an entrypoint
	Commands []string
}

// PodmanCliCommandBuilder allows you to build the podman command in a
// way that is already unit tested and verified so that you do not have to
// append your own strings or do variable interpolation.
type PodmanCliCommandBuilder struct {
	parts CliParts
}

// WithPath allows you to override the default path of /usr/local/bin/podman
// for podman.
func (b *PodmanCliCommandBuilder) WithPath(path string) *PodmanCliCommandBuilder {
	b.parts.Path = path
	return b
}

// WithImage specifies the container image used in the command.
func (b *PodmanCliCommandBuilder) WithImage(imageName string) *PodmanCliCommandBuilder {
	b.parts.Image = imageName
	return b
}

// WithWorkspace is a shortcut to adding a local path to the "workspace" on
// the container.
func (b *PodmanCliCommandBuilder) WithWorkspace(localdir string) *PodmanCliCommandBuilder {
	return b.WithVolume(localdir, b.parts.Workdir)
}

// WithVolume adds a volume mapping to the command.
func (b *PodmanCliCommandBuilder) WithVolume(localdir string, containerdir string) *PodmanCliCommandBuilder {
	return b.WithVolumeOpt(localdir, containerdir, "")
}

// WithVolume adds a volume mapping to the command.
func (b *PodmanCliCommandBuilder) WithVolumeOpt(localdir string, containerdir string, option string) *PodmanCliCommandBuilder {
	var volMap string
	if len(option) > 0 {
		volMap = fmt.Sprintf("%s:%s:%s", localdir, containerdir, option)
	} else {
		volMap = fmt.Sprintf("%s:%s", localdir, containerdir)
	}
	b.parts.VolumeMaps = append(b.parts.VolumeMaps, volMap)
	return b
}

func (b *PodmanCliCommandBuilder) WithUserMap(localUser int, containerUser int, number int) *PodmanCliCommandBuilder {
	mapstr := fmt.Sprintf("%d:%d:%d", containerUser, localUser, number)
	b.parts.UidMaps = append(b.parts.UidMaps, mapstr)
	return b
}

// WithPort adds a port mapping to the command
func (b *PodmanCliCommandBuilder) WithPort(localport string, containerport string) *PodmanCliCommandBuilder {
	b.parts.Ports[localport] = containerport
	return b
}

// WithEnvvar adds the given environment variable and value to the command.
// It is the same thing as adding -e ENVAR=value as a parameter to the
// container command.
func (b *PodmanCliCommandBuilder) WithEnvvar(name string, value string) *PodmanCliCommandBuilder {
	envar := &EnvVarInfo{
		Name:  name,
		Value: value,
	}
	b.parts.Envvars = append(b.parts.Envvars, *envar)
	return b
}

// Build builds the command line for the container command
func (b *PodmanCliCommandBuilder) Build() (string, error) {
	buf := new(bytes.Buffer)
	tmpl, err := template.New("cli").Parse("{{.Path}} {{.Cmd}}{{- range .Flags}} {{.}}{{end}}{{- range .UidMaps}} --uidmap {{.}}{{end}}{{- range .VolumeMaps}} -v {{.}}{{end}}{{- range $k,$v := .Ports}} -p {{$k}}:{{$v}}{{end}}{{range .Envvars}} -e {{.}}{{end}}{{if .Image}} {{.Image}}{{end}}")
	if err != nil {
		// This template is hardcoded here, so if it does not parse properly,
		// we want the developer to know write away.
		panic(err)
	}
	tmpl.Execute(buf, b.parts)
	return strings.TrimSpace(buf.String()), nil

}

func (b *PodmanCliCommandBuilder) BuildFrom(info ImageInfo) (string, error) {
	b.WithImage(info.Image)
	for _, envvar := range info.EnvVars {
		b.WithEnvvar(envvar.Name, envvar.Value)
	}
	return b.Build()
}

// NewPodmanCliCommandBuilder creates a new PodmanCliCommandBuilder
// with the given configuration. If there is no configuration provided
// (nil), or if certain values are not defined, then the constructor
// will provide reasonable defaults.
func NewPodmanCliCommandBuilder(cli *CliParts) *PodmanCliCommandBuilder {
	defaults := cli
	if defaults == nil {
		defaults = &CliParts{}
	}
	defaultFlags := make([]string, 0)
	parts := &CliParts{
		Path:             Iif(defaults.Path, "/usr/local/bin/podman"),
		Cmd:              Iif(defaults.Cmd, "run"),
		Workdir:          Iif(defaults.Workdir, "/workspace"),
		Flags:            append(defaults.Flags, defaultFlags...),
		Envvars:          defaults.Envvars,
		DefaultVolumeOpt: "Z",
		VolumeMaps:       make([]string, 0),
		Ports:            make(map[string]string, 0),
		UidMaps:          make([]string, 0),
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
	In          io.Reader
	Out         io.Writer
	Log         logger.Logger
	Err         io.Writer
	Errors      []error
	LastErrCode int
}

// AddError adds an error to the context
func (c *RunContext) AddError(err error) {
	if c.Errors == nil {
		c.Errors = make([]error, 0)
	}
	c.Errors = append(c.Errors, err)
}

func (c *RunContext) Reset() {
	c.LastErrCode = 0
}

func (c *RunContext) SetLastErrCode(errCode int) {
	c.LastErrCode = errCode
}

// IsErrored returns true if there are errors in the context
func (c *RunContext) IsErrored() bool {
	return len(c.Errors) > 0 || c.LastErrCode != 0
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
		if exiterr, ok := err.(*exec.ExitError); ok {
			ctx.SetLastErrCode(exiterr.ExitCode())
		}
		ctx.AddError(err)
	}
	return err
}

// RunImage runs the container that is defined in the provided ImageInfo
func (r *CliModuleRunner) RunImage(ctx *RunContext, info ImageInfo) error {
	cmdStr, err := r.BuildFrom(info)
	if err != nil {
		ctx.AddError(err)
		return err
	}

	return r.runCmd(ctx, cmdStr)
}

// Run runs the container that has been defined in the builder setup.
func (r *CliModuleRunner) Run(ctx *RunContext) error {
	cmdStr, err := r.Build()
	if err != nil {
		ctx.AddError(err)
		return err
	}
	// Immediately before we run, we reset the context
	ctx.Reset()
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
	builder = builder.WithWorkspace(cwd)

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
