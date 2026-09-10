package tui

import (
	"context"

	"conan-cli/internal/config"
	"conan-cli/internal/workflow"
)

// tuiAPI 是交互式 TUI 需要的全部 workflow 能力。生产实现直接包住
// *workflow.App；测试注入 fake，保证不依赖真实 Conan 与网络。
type tuiAPI interface {
	Init(ctx context.Context) (workflow.Report, error)
	Scan(ctx context.Context) (workflow.Report, error)
	Analyze(ctx context.Context, osName, arch, buildType string) (workflow.Report, error)
	InstallPlatform(ctx context.Context, request workflow.InstallRequest) (workflow.Report, error)
	PublishPackage(ctx context.Context, request workflow.PublishRequest) (workflow.Report, error)
	Add(dependency string) (workflow.Report, error)
	GenerateRecipe(kind string, force bool, name, version, qt string) (workflow.Report, error)
	CatalogFilter(ctx context.Context, filter workflow.CatalogFilter) (workflow.Report, error)
	Doctor(ctx context.Context) (workflow.Report, error)
	Status(ctx context.Context) (workflow.Report, error)
	Project() (*config.Project, error)
	SaveProjectSettings(input workflow.ProjectSettingsInput) (workflow.Report, error)
	SaveGlobalSettings(ctx context.Context, input workflow.GlobalSettingsInput) (workflow.Report, error)
	ConfigLogin(ctx context.Context, password string) (workflow.Report, error)
	ConfigTest(ctx context.Context) (workflow.Report, error)
}

type btAppAPI struct {
	app *workflow.App
}

func (a btAppAPI) Init(ctx context.Context) (workflow.Report, error) {
	return a.app.Init(ctx)
}

func (a btAppAPI) Scan(ctx context.Context) (workflow.Report, error) {
	return a.app.Scan(ctx)
}

func (a btAppAPI) Analyze(ctx context.Context, osName, arch, buildType string) (workflow.Report, error) {
	return a.app.Analyze(ctx, osName, arch, buildType)
}

func (a btAppAPI) InstallPlatform(ctx context.Context, request workflow.InstallRequest) (workflow.Report, error) {
	return a.app.InstallPlatform(ctx, request)
}

func (a btAppAPI) PublishPackage(ctx context.Context, request workflow.PublishRequest) (workflow.Report, error) {
	return a.app.PublishPackage(ctx, request)
}

func (a btAppAPI) Add(dependency string) (workflow.Report, error) {
	return a.app.Add(dependency)
}

func (a btAppAPI) GenerateRecipe(kind string, force bool, name, version, qt string) (workflow.Report, error) {
	return a.app.GenerateRecipe(kind, force, name, version, qt)
}

func (a btAppAPI) CatalogFilter(ctx context.Context, filter workflow.CatalogFilter) (workflow.Report, error) {
	return a.app.CatalogFilter(ctx, filter)
}

func (a btAppAPI) Doctor(ctx context.Context) (workflow.Report, error) {
	return a.app.Doctor(ctx)
}

func (a btAppAPI) Status(ctx context.Context) (workflow.Report, error) {
	return a.app.Status(ctx)
}

func (a btAppAPI) Project() (*config.Project, error) {
	return a.app.Project()
}

func (a btAppAPI) SaveProjectSettings(input workflow.ProjectSettingsInput) (workflow.Report, error) {
	return a.app.SaveProjectSettings(input)
}

func (a btAppAPI) SaveGlobalSettings(ctx context.Context, input workflow.GlobalSettingsInput) (workflow.Report, error) {
	return a.app.SaveGlobalSettings(ctx, input)
}

func (a btAppAPI) ConfigLogin(ctx context.Context, password string) (workflow.Report, error) {
	return a.app.ConfigLogin(ctx, password)
}

func (a btAppAPI) ConfigTest(ctx context.Context) (workflow.Report, error) {
	return a.app.ConfigTest(ctx)
}
