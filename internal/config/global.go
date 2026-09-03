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

type Global struct {
	Nexus    Nexus  `yaml:"nexus" json:"nexus"`
	ConanBin string `yaml:"conan_bin,omitempty" json:"conan_bin,omitempty"`
	CLIBin   string `yaml:"cli_bin,omitempty" json:"cli_bin,omitempty"`
}

type Nexus struct {
	Name     string `yaml:"name,omitempty" json:"name,omitempty"`
	URL      string `yaml:"url,omitempty" json:"url,omitempty"`
	Username string `yaml:"username,omitempty" json:"username,omitempty"`
}

type GlobalView struct {
	Nexus       Nexus  `json:"nexus"`
	ConanBin    string `json:"conan_bin,omitempty"`
	CLIBin      string `json:"cli_bin,omitempty"`
	HasPassword bool   `json:"has_password"`
	Path        string `json:"path"`
}

func HomeDir() string {
	if override := strings.TrimSpace(os.Getenv("CONAN_CLI_HOME")); override != "" {
		return override
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ".conan-cli"
	}
	return filepath.Join(home, ".conan-cli")
}

func GlobalPath() string {
	return filepath.Join(HomeDir(), "config.yaml")
}

func CredentialsPath() string {
	return filepath.Join(HomeDir(), "credentials")
}

func LoadGlobal() (*Global, error) {
	path := GlobalPath()
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return &Global{}, nil
		}
		return nil, fmt.Errorf("read global config: %w", err)
	}
	var global Global
	if err := yaml.Unmarshal(data, &global); err != nil {
		return nil, fmt.Errorf("parse global config: %w", err)
	}
	if global.Nexus.Name == "" {
		global.Nexus.Name = "nexus"
	}
	return &global, nil
}

func SaveGlobal(global *Global) error {
	if global == nil {
		return errors.New("global config is nil")
	}
	if global.Nexus.Name == "" {
		global.Nexus.Name = "nexus"
	}
	data, err := yaml.Marshal(global)
	if err != nil {
		return fmt.Errorf("encode global config: %w", err)
	}
	if err := os.MkdirAll(HomeDir(), 0o700); err != nil {
		return fmt.Errorf("create global config directory: %w", err)
	}
	return atomicfile.Write(GlobalPath(), data, 0o600)
}

func LoadPassword() (string, error) {
	data, err := os.ReadFile(CredentialsPath())
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", nil
		}
		return "", fmt.Errorf("read credentials: %w", err)
	}
	return strings.TrimSpace(string(data)), nil
}

func SavePassword(password string) error {
	if err := os.MkdirAll(HomeDir(), 0o700); err != nil {
		return fmt.Errorf("create global config directory: %w", err)
	}
	password = strings.TrimSpace(password)
	if password == "" {
		_ = os.Remove(CredentialsPath())
		return nil
	}
	return atomicfile.Write(CredentialsPath(), []byte(password+"\n"), 0o600)
}

func (g *Global) View() GlobalView {
	password, _ := LoadPassword()
	path := GlobalPath()
	if g == nil {
		return GlobalView{Path: path, Nexus: Nexus{Name: "nexus"}}
	}
	return GlobalView{
		Nexus:       g.Nexus,
		ConanBin:    g.ConanBin,
		CLIBin:      g.CLIBin,
		HasPassword: password != "",
		Path:        path,
	}
}
