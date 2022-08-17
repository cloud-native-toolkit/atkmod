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

type AtkModule struct {
	Id             string   `json:"id" yaml:"id"`
	Name           string   `json:"name" yaml:"name"`
	Version        string   `json:"version" yaml:"version"`
	TemplateUrl    string   `json:"template_url" yaml:"template_url"`
	Facets         []string `json:"facets" yaml:"facets"`
	Dependencies   []string `json:"dependencies" yaml:"dependencies"`
	Meta           MetaInfo `json:"meta" yaml:"meta"`
	Specifications SpecInfo `json:"spec" yaml:"spec"`
}

type AtkModuleRunner interface {
	ListParams() ([]string, error)
	ValidateParam(name string, value string) (bool, error)
	PreDeploy(ctx context.Context) error
	Deploy(ctx context.Context) error
	PostDeploy(ctx context.Context) error
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

type AtkRunCfg struct {
	Stdout io.Writer
	Stderr io.Writer
	Logger *logger.Logger
}

type CliModuleRunner struct {
	AtkRunCfg
	PodmanCliCommandBuilder
}

// RunImage
func (r *CliModuleRunner) Run(ctx context.Context, info ImageInfo) error {
	cmdStr, err := r.BuildFrom(info)
	if err != nil {
		return err
	}

	r.Logger.Infof("running command: %s", cmdStr)
	// TODO: Here we will actually run the command.
	cmdParts := strings.Split(cmdStr, " ")
	runCmd := exec.Command(cmdParts[0], cmdParts[1:]...)
	runCmd.Stdout = r.Stdout
	runCmd.Stderr = r.Stderr

	err = runCmd.Run()

	return err
}

type Status string

const (
	None          Status = "none"
	Invalid       Status = "invalid"
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

type RunFunc func()

type ModState struct {
	Previous Status
	Current  Status
	Handler  RunFunc
	Next     Status
}

type AtkDepoyableModule struct {
	AtkModule
	ModState
	CliModuleRunner
}

func (m *AtkDepoyableModule) updateState(state Status) {
	m.Previous = m.Current
	m.Current = state
}

func (m *AtkDepoyableModule) ListParams() ([]string, error) { return []string{""}, nil }

func (m *AtkDepoyableModule) ValidateParam(name string, value string) (bool, error) {
	return false, nil
}

func (m *AtkDepoyableModule) PreDeploy(ctx context.Context) error {
	m.updateState(PreDeploying)
	err := m.Run(ctx, m.Specifications.PreDeploy)
	if err != nil {
		m.updateState(Errored)
	} else {
		m.updateState(PreDeployed)
	}
	return err
}

func (m *AtkDepoyableModule) Deploy(ctx context.Context) error {
	m.updateState(Deploying)
	err := m.Run(ctx, m.Specifications.Deploy)
	if err != nil {
		m.updateState(Errored)
	} else {
		m.updateState(Deployed)
	}
	return err
}

func (m *AtkDepoyableModule) PostDeploy(ctx context.Context) error {
	m.updateState(PostDeploying)
	err := m.Run(ctx, m.Specifications.PostDeploy)
	if err != nil {
		m.updateState(Errored)
	} else {
		m.updateState(PostDeployed)
	}
	return err
}

func (m *AtkDepoyableModule) IsErrored() bool {
	return m.Current == Errored
}

func NewAtkDeployableModule(ctx context.Context, runCfg *AtkRunCfg, module *AtkModule) *AtkDepoyableModule {
	builder := NewPodmanCliCommandBuilder()
	cwd := fmt.Sprintf("%s", ctx.Value(BaseDirectory))
	if len(cwd) == 0 {
		cwd, _ = os.Getwd()
	}
	builder = builder.WithVolume(cwd)
	runner := &CliModuleRunner{
		*runCfg,
		*builder,
	}
	obj := &AtkDepoyableModule{
		*module,
		ModState{
			Previous: None,
			Current:  Invalid,
		},
		*runner,
	}
	return obj
}

type AtkModuleLoader interface {
	Load(uri string) (AtkModule, error)
}

type AtkManifestFileLoader struct {
	path string
}

func (l *AtkManifestFileLoader) Load(uri string) (*AtkModule, error) {
	l.path = uri
	logger.Debug("Loading module from manifest file")
	var module *AtkModule = &AtkModule{}
	yamlFile, err := ioutil.ReadFile(uri)
	if err != nil {
		return nil, err
	}
	err = yaml.Unmarshal(yamlFile, &module)
	return module, err

}

func NewAtkManifestFileLoader() *AtkManifestFileLoader {
	return &AtkManifestFileLoader{}
}
