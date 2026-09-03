package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"conan-cli/internal/atomicfile"

	"gopkg.in/yaml.v3"
)

const (
	ProjectConfigPath   = ".conan-cli/project.yaml"
	DefaultOutputFolder = "conan"
)

type Compiler struct {
	ID      string `yaml:"id,omitempty" json:"id,omitempty"`
	Version string `yaml:"version,omitempty" json:"version,omitempty"`
}

func (c Compiler) Display() string {
	c.ID = strings.TrimSpace(c.ID)
	c.Version = strings.TrimSpace(c.Version)
	if c.ID == "" && c.Version == "" {
		return ""
	}
	if c.Version == "" {
		return c.ID
	}
	if c.ID == "" {
		return c.Version
	}
	return c.ID + " " + c.Version
}

type PlatformSpec struct {
	OS        string `yaml:"os,omitempty" json:"os,omitempty"`
	Arch      string `yaml:"arch,omitempty" json:"arch,omitempty"`
	BuildType string `yaml:"build_type,omitempty" json:"build_type,omitempty"`
}

func (p PlatformSpec) Empty() bool {
	return strings.TrimSpace(p.OS) == "" && strings.TrimSpace(p.Arch) == ""
}

func (p PlatformSpec) Display() string {
	osName := DisplayOS(p.OS)
	arch := DisplayArch(p.Arch)
	if osName == "" && arch == "" {
		return ""
	}
	if osName == "" {
		osName = "-"
	}
	if arch == "" {
		arch = "-"
	}
	out := osName + " / " + arch
	if bt := DisplayBuildType(p.BuildType); bt != "" {
		out += " · " + bt
	}
	return out
}

type Platform struct {
	Consume PlatformSpec `yaml:"consume" json:"consume"`
	Publish PlatformSpec `yaml:"publish" json:"publish"`
}

type Project struct {
	Name                string   `yaml:"name" json:"name"`
	NameLocked          bool     `yaml:"name_locked,omitempty" json:"name_locked,omitempty"`
	BuildSystem         string   `yaml:"build_system" json:"build_system"`
	DefaultProfile      string   `yaml:"default_profile,omitempty" json:"default_profile,omitempty"`
	QtVersion           string   `yaml:"qt_version,omitempty" json:"qt_version,omitempty"`
	Compiler            Compiler `yaml:"compiler,omitempty" json:"compiler"`
	Platform            Platform `yaml:"platform,omitempty" json:"platform"`
	Remote              string   `yaml:"remote,omitempty" json:"remote,omitempty"`
	Channel             string   `yaml:"channel" json:"channel"`
	MissingBinaryPolicy string   `yaml:"missing_binary_policy" json:"missing_binary_policy"`
	OutputFolder        string   `yaml:"output_folder" json:"output_folder"`
	Dependencies        []string `yaml:"dependencies,omitempty" json:"dependencies,omitempty"`
}

func ProjectPath(dir string) string {
	return filepath.Join(dir, ProjectConfigPath)
}

func LoadProject(dir string) (*Project, error) {
	data, err := os.ReadFile(ProjectPath(dir))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("project is not initialized; run 'conan-cli init'")
		}
		return nil, fmt.Errorf("read project config: %w", err)
	}

	var project Project
	if err := yaml.Unmarshal(data, &project); err != nil {
		return nil, fmt.Errorf("parse project config: %w", err)
	}
	applyDefaults(&project, filepath.Base(dir))
	if err := ValidateProject(&project); err != nil {
		return nil, err
	}
	return &project, nil
}

func SaveProject(dir string, project *Project) error {
	if project == nil {
		return errors.New("project config is nil")
	}
	applyDefaults(project, filepath.Base(dir))
	if err := ValidateProject(project); err != nil {
		return err
	}
	data, err := yaml.Marshal(project)
	if err != nil {
		return fmt.Errorf("encode project config: %w", err)
	}
	path := ProjectPath(dir)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create project config directory: %w", err)
	}
	if err := atomicfile.Write(path, data, 0o644); err != nil {
		return fmt.Errorf("write project config: %w", err)
	}
	return nil
}

func NewProject(dir string) *Project {
	name := filepath.Base(dir)
	project := &Project{Name: name}
	if _, err := os.Stat(filepath.Join(dir, "CMakeLists.txt")); err == nil {
		project.BuildSystem = "cmake"
	} else if matches, _ := filepath.Glob(filepath.Join(dir, "*.pro")); len(matches) > 0 {
		project.BuildSystem = "qmake"
	} else {
		project.BuildSystem = "unknown"
	}
	applyDefaults(project, name)
	return project
}

func AddDependency(project *Project, dependency string) error {
	if project == nil {
		return errors.New("project config is nil")
	}
	dependency = strings.TrimSpace(dependency)
	if dependency == "" {
		return errors.New("package reference cannot be empty")
	}
	for _, existing := range project.Dependencies {
		if existing == dependency {
			return nil
		}
	}
	project.Dependencies = append(project.Dependencies, dependency)
	return nil
}

func ValidateProject(project *Project) error {
	if project == nil {
		return errors.New("project config is nil")
	}
	if project.MissingBinaryPolicy != "download-only" {
		return fmt.Errorf("unsupported missing_binary_policy %q; only download-only is supported", project.MissingBinaryPolicy)
	}
	for index, dependency := range project.Dependencies {
		if strings.TrimSpace(dependency) == "" {
			return fmt.Errorf("dependency %d is empty", index)
		}
	}
	if err := ValidatePlatformSpec(project.Platform.Consume); err != nil {
		return fmt.Errorf("consume platform: %w", err)
	}
	if err := ValidatePlatformSpec(project.Platform.Publish); err != nil {
		return fmt.Errorf("publish platform: %w", err)
	}
	return nil
}

func ValidatePlatformSpec(spec PlatformSpec) error {
	if spec.Empty() {
		return nil
	}
	if spec.OS != "" && !ValidOS(spec.OS) {
		return fmt.Errorf("unsupported os %q; use windows, linux, or kylin", spec.OS)
	}
	if spec.Arch != "" && !ValidArch(spec.Arch) {
		return fmt.Errorf("unsupported arch %q; use x86, x64, arm, or arm64", spec.Arch)
	}
	if spec.BuildType != "" && !ValidBuildType(spec.BuildType) {
		return fmt.Errorf("unsupported build_type %q; use Debug or Release", spec.BuildType)
	}
	return nil
}

func applyDefaults(project *Project, fallbackName string) {
	if project.Name == "" {
		project.Name = fallbackName
	}
	if project.Channel == "" {
		project.Channel = "dev"
	}
	if project.DefaultProfile == "" {
		project.DefaultProfile = "default"
	}
	if project.MissingBinaryPolicy == "" {
		project.MissingBinaryPolicy = "download-only"
	}
	if project.OutputFolder == "" {
		project.OutputFolder = DefaultOutputFolder
	}
	project.Platform.Consume.OS = NormalizeOS(project.Platform.Consume.OS)
	project.Platform.Consume.Arch = NormalizeArch(project.Platform.Consume.Arch)
	project.Platform.Consume.BuildType = NormalizeBuildType(project.Platform.Consume.BuildType)
	project.Platform.Publish.OS = NormalizeOS(project.Platform.Publish.OS)
	project.Platform.Publish.Arch = NormalizeArch(project.Platform.Publish.Arch)
	project.Platform.Publish.BuildType = NormalizeBuildType(project.Platform.Publish.BuildType)
	if project.Platform.Publish.Empty() {
		project.Platform.Publish = project.Platform.Consume
	}
}
