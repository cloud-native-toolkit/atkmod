package atkmod

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"
	"text/template"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	logger "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
)

type AtkContextKey string

type ModuleEventType string
type Hook string

const (
	apiVersionSeparator = "/"
	apiName             = "itzcli"
	apiVersionv1Alpha1  = "v1alpha1"
	installKind         = "InstallManifest"

	ListHookResponseEvent           ModuleEventType = "com.ibm.techzone.cli.hook.list.response"
	ValidateHookResponseEvent       ModuleEventType = "com.ibm.techzone.cli.hook.validate.response"
	ValidateHookRequestEvent        ModuleEventType = "com.ibm.techzone.cli.hook.validate.request"
	GetStateHookResponseEvent       ModuleEventType = "com.ibm.techzone.cli.hook.get_state.response"
	GetStateHookRequestEvent        ModuleEventType = "com.ibm.techzone.cli.hook.get_state.request"
	PreDeployLifecycleRequestEvent  ModuleEventType = "com.ibm.techzone.cli.lifecycle.pre_deploy.request"
	DeployLifecycleRequestEvent     ModuleEventType = "com.ibm.techzone.cli.lifecycle.deploy.request"
	PostDeployLifecycleRequestEvent ModuleEventType = "com.ibm.techzone.cli.lifecycle.post_deploy.request"
	LoggerContextKey                AtkContextKey   = "atk.logger"
	StdOutContextKey                AtkContextKey   = "atk.stdout"
	StdErrContextKey                AtkContextKey   = "atk.stderr"
	BaseDirectory                   AtkContextKey   = "atk.basedir"
	ListHook                        Hook            = "list"
	ValidateHook                    Hook            = "validate"
	GetStateHook                    Hook            = "get_state"
)

var (
	supportedAPIVersions = []string{apiVersionv1Alpha1}
)

type EventDataVarInfo struct {
	Name        string `json:"name" yaml:"name"`
	Value       string `json:"value,omitempty" yaml:"value,omitempty"`
	Default     string `json:"default,omitempty" yaml:"default,omitempty"`
	Description string `json:"description,omitempty" yaml:"description,omitempty"`
}

type EventData struct {
	Variables []EventDataVarInfo `json:"variables,omitempty" yaml:"variables,omitempty"`
}

type EnvVarInfo struct {
	Name  string `json:"name" yaml:"name"`
	Value string `json:"value" yaml:"value"`
}

func (e *EnvVarInfo) String() string {
	return fmt.Sprintf("%s=%s", e.Name, e.Value)
}

type VolumeInfo struct {
	MountPath string `json:"mountPath" yaml:"mountPath"`
	Name      string `json:"name" yaml:"name"`
}

type ImageInfo struct {
	Image   string       `json:"image" yaml:"image"`
	Script  string       `json:"script" yaml:"script"`
	Command []string     `json:"command" yaml:"command"`
	Args    []string     `json:"args" yaml:"args"`
	EnvVars []EnvVarInfo `json:"env" yaml:"env"`
	Volumes []VolumeInfo `json:"volumeMounts" yaml:"volumeMounts"`
}

type HookInfo struct {
	GetState ImageInfo `json:"get_state" yaml:"get_state"`
	List     ImageInfo `json:"list" yaml:"list"`
	Validate ImageInfo `json:"validate" yaml:"validate"`
}

type MetadataInfo struct {
	Name      string            `json:"name" yaml:"name"`
	Namespace string            `json:"namespace" yaml:"namespace"`
	Labels    map[string]string `json:"labels" yaml:"labels"`
}
type LifecycleInfo struct {
	PreDeploy  ImageInfo `json:"pre_deploy" yaml:"pre_deploy"`
	Deploy     ImageInfo `json:"deploy" yaml:"deploy"`
	PostDeploy ImageInfo `json:"post_deploy" yaml:"post_deploy"`
}

type SpecInfo struct {
	Hooks     HookInfo      `json:"hooks" yaml:"hooks"`
	Lifecycle LifecycleInfo `json:"lifecycle" yaml:"lifecycle"`
}

type ApiVersion struct {
	Namespace string
	Version   string
}

func (a ApiVersion) String() string {
	return fmt.Sprintf("%s%s%s", a.Namespace, apiVersionSeparator, a.Version)
}

// ParseApiVersion parses the version of the apiVersion
func ParseApiVersion(val string) (*ApiVersion, error) {
	parts := strings.Split(val, apiVersionSeparator)
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid apiVersion format: %s", val)
	}

	return &ApiVersion{Namespace: parts[0], Version: parts[1]}, nil
}

type ModuleInfo struct {
	ApiVersion     string       `json:"apiVersion" yaml:"apiVersion"`
	Kind           string       `json:"kind" yaml:"kind"`
	Metadata       MetadataInfo `json:"metadata" yaml:"metadata"`
	Specifications SpecInfo     `json:"spec" yaml:"spec"`
}

// IsSupportedKind returns true if the kind is supported.
func (m *ModuleInfo) IsSupportedKind() bool {
	return m.Kind == installKind
}

// IsSupportedVersion returns true if apiVersion is supported.
func (m *ModuleInfo) IsSupportedVersion() bool {
	ver, err := ParseApiVersion(m.ApiVersion)
	if err != nil {
		return false
	}
	if ver.Namespace != apiName {
		return false
	}
	for _, version := range supportedAPIVersions {
		if ver.Version == version {
			return true
		}
	}
	return false
}

// IsSupported returns true if the manifest file is supported.
func (m *ModuleInfo) IsSupported() bool {
	return m.IsSupportedKind() && m.IsSupportedVersion()
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
	// TODO: this should go away once this is supported, but for now we want
	// to make sure we tell the user.
	if len(info.Command) > 0 {
		return "", errors.New("command is not yet supported")
	}

	b.WithImage(info.Image)
	for _, envvar := range info.EnvVars {
		b.WithEnvvar(envvar.Name, envvar.Value)
	}
	for _, v := range info.Volumes {
		b.WithVolume(v.Name, v.MountPath)
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
		defaults.Path = os.Getenv("ITZ_PODMAN_PATH")
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
	Context     context.Context
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
	runCmd.Stdin = ctx.In
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
	Done                = PostDeployed
	Errored       State = "errored"
)

var DefaultOrder = []State{
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

type HookCmd func(ctx *RunContext) error

// NoopHandler is an implementation of the Null Object pattern.
// It does nothing except to insure we don't return a nil.
func NoopHandler(ctx *RunContext, notifier Notifier) error {
	notifier.Notify(Invalid)
	return nil
}

func NoopHookCmd(ctx *RunContext) error {
	return nil
}

func DoneHandler(ctx *RunContext, notifier Notifier) error {
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
	hooks     map[Hook]HookCmd
	previous  State
	current   State
	execOrder []State
}

func (m *DeployableModule) getHookCmd(img ImageInfo) HookCmd {
	return func(ctx *RunContext) error {
		return m.cli.RunImage(ctx, img)
	}
}

func (m *DeployableModule) addHook(name Hook, hook HookCmd) error {
	m.hooks[name] = hook
	return nil
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
	m.runCtx.Log.Tracef("Adding command for: %s", status)
	if m.cmds[status] == nil {
		m.cmds[status] = handler
		return nil
	} else {
		return fmt.Errorf("handler for state %s already exists", status)
	}
}

func (m *DeployableModule) GetCmdFor(status State) StateCmd {
	m.runCtx.Log.Tracef("Getting command for: %s", status)
	return m.cmds[status]
}

func (m *DeployableModule) GetHook(name Hook) HookCmd {
	m.runCtx.Log.Tracef("Getting hook for: %s", name)
	return m.hooks[name]
}

type NextFunc func() (StateCmd, bool)

func (m *DeployableModule) Itr() (NextFunc, bool) {
	return func() (StateCmd, bool) {
		if m.current == Done {
			return DoneHandler, false
		}
		if m.current == Errored {
			return DoneHandler, false
		}

		for idx, state := range m.execOrder {
			if m.current == state {
				m.runCtx.Log.Tracef("Found state: %s; next state is: %s", m.current, m.execOrder[idx+1])
				return m.GetCmdFor(m.execOrder[idx]), true
			}
		}
		return NoopHandler, false
	}, true
}

func (m *DeployableModule) preDeploy(ctx *RunContext, notifier Notifier) error {
	notifier.Notify(PreDeploying)
	err := m.cli.RunImage(ctx, m.module.Specifications.Lifecycle.PreDeploy)
	if err != nil {
		notifier.Notify(Errored)
	} else {
		notifier.Notify(PreDeployed)
	}
	return err
}

func (m *DeployableModule) deploy(ctx *RunContext, notifier Notifier) error {
	notifier.Notify(Deploying)
	err := m.cli.RunImage(ctx, m.module.Specifications.Lifecycle.Deploy)
	if err != nil {
		notifier.Notify(Errored)
	} else {
		notifier.Notify(Deployed)
	}
	return err
}

func (m *DeployableModule) postDeploy(ctx *RunContext, notifier Notifier) error {
	notifier.Notify(PostDeploying)
	err := m.cli.RunImage(ctx, m.module.Specifications.Lifecycle.PostDeploy)
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

func NewDeployableModule(runCtx *RunContext, module *ModuleInfo) *DeployableModule {
	builder := NewPodmanCliCommandBuilder(nil)

	deployment := &DeployableModule{
		module:    module,
		cli:       &CliModuleRunner{*builder},
		runCtx:    *runCtx,
		execOrder: DefaultOrder,
		current:   Invalid,
		cmds:      make(map[State]StateCmd),
		hooks:     make(map[Hook]HookCmd),
	}

	deployment.addHook(ListHook, deployment.getHookCmd(module.Specifications.Hooks.List))
	deployment.addHook(ValidateHook, deployment.getHookCmd(module.Specifications.Hooks.Validate))
	deployment.addHook(GetStateHook, deployment.getHookCmd(module.Specifications.Hooks.GetState))

	// Now configure the cmds for the module deployment
	deployment.AddCmd(Invalid, advanceTo(Initializing))
	deployment.AddCmd(Initializing, deployment.resolveState)
	deployment.AddCmd(Configured, advanceTo(Validated))
	deployment.AddCmd(Validated, advanceTo(PreDeploying))
	deployment.AddCmd(PreDeploying, deployment.preDeploy)
	deployment.AddCmd(PreDeployed, advanceTo(Deploying))
	deployment.AddCmd(Deploying, deployment.deploy)
	deployment.AddCmd(Deployed, advanceTo(PostDeploying))
	deployment.AddCmd(PostDeploying, deployment.postDeploy)
	deployment.AddCmd(PostDeployed, advanceTo(Done))

	return deployment
}

func advanceTo(s State) StateCmd {
	return func(ctx *RunContext, notifier Notifier) error {
		notifier.Notify(s)
		return nil
	}
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
	var module = &ModuleInfo{}
	yamlFile, err := ioutil.ReadFile(uri)
	if err != nil {
		return nil, err
	}
	err = yaml.Unmarshal(yamlFile, &module)
	if err != nil {
		return nil, err
	}
	// Now check to make sure the module is a supported version
	supported := module.IsSupported()
	if !supported {
		err = fmt.Errorf("module version %s is not supported", module.ApiVersion)
	}
	return module, err
}

func NewAtkManifestFileLoader() *ManifestFileLoader {
	return &ManifestFileLoader{}
}

func LoadEventData(event *cloudevents.Event) (*EventData, error) {
	var data EventData
	err := yaml.Unmarshal(event.Data(), &data)
	return &data, err
}

func LoadEvent(eventS string) (*cloudevents.Event, error) {
	event := cloudevents.NewEvent()
	err := json.Unmarshal([]byte(eventS), &event)
	if err != nil {
		return nil, err
	}
	return &event, nil
}

func WriteEvent(event *cloudevents.Event, out io.Writer) error {
	bytes, err := json.Marshal(event)
	if err != nil {
		return err
	}
	_, err = out.Write(bytes)
	return err
}
