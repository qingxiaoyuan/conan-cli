package workflow

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"

	"conan-cli/internal/config"
)

type ProjectSettingsInput struct {
	Name             string
	QtVersion        string
	CompilerID       string
	CompilerVersion  string
	OS               string
	Arch             string
	BuildType        string
	PublishOS        string
	PublishArch      string
	PublishBuildType string
	Channel          string
	Remote           string
	BuildSystem      string
	OutputFolder     string
}

type GlobalSettingsInput struct {
	Name     string
	URL      string
	Username string
	Password string
	ConanBin string
}

func (a *App) ShowSettings() (Report, error) {
	project, _ := a.Project()
	global, _ := config.LoadGlobal()
	if global == nil {
		global = &config.Global{}
	}
	return Report{OK: true, Action: "settings", Data: map[string]any{
		"project": project,
		"global":  global.View(),
	}}, nil
}

func (a *App) SaveProjectSettings(input ProjectSettingsInput) (Report, error) {
	project, err := a.ensureProject()
	if err != nil {
		return Report{}, err
	}
	if input.Name != "" {
		project.Name = input.Name
	}
	if input.QtVersion != "" {
		project.QtVersion = input.QtVersion
	}
	if input.CompilerID != "" {
		project.Compiler.ID = input.CompilerID
	}
	if input.CompilerVersion != "" {
		project.Compiler.Version = input.CompilerVersion
	}
	if input.OS != "" {
		project.Platform.Consume.OS = input.OS
	}
	if input.Arch != "" {
		project.Platform.Consume.Arch = input.Arch
	}
	if input.BuildType != "" {
		project.Platform.Consume.BuildType = config.NormalizeBuildType(input.BuildType)
	}
	if input.PublishOS != "" {
		project.Platform.Publish.OS = input.PublishOS
	}
	if input.PublishArch != "" {
		project.Platform.Publish.Arch = input.PublishArch
	}
	if input.PublishBuildType != "" {
		project.Platform.Publish.BuildType = config.NormalizeBuildType(input.PublishBuildType)
	}
	if project.Platform.Publish.Empty() {
		project.Platform.Publish = project.Platform.Consume
	}
	if input.Channel != "" {
		project.Channel = input.Channel
	}
	if input.Remote != "" {
		project.Remote = input.Remote
	}
	if input.BuildSystem != "" {
		project.BuildSystem = input.BuildSystem
	}
	if input.OutputFolder != "" {
		project.OutputFolder = input.OutputFolder
	}
	if err := config.SaveProject(a.Dir, project); err != nil {
		return Report{}, err
	}
	return Report{OK: true, Action: "settings-project", Message: "项目设置已保存", Data: project}, nil
}

func (a *App) SaveGlobalSettings(ctx context.Context, input GlobalSettingsInput) (Report, error) {
	global, err := config.LoadGlobal()
	if err != nil {
		return Report{}, err
	}
	if input.Name != "" {
		global.Nexus.Name = input.Name
	}
	if input.URL != "" {
		global.Nexus.URL = input.URL
	}
	if input.Username != "" {
		global.Nexus.Username = input.Username
	}
	if input.ConanBin != "" {
		global.ConanBin = input.ConanBin
	}
	if err := config.SaveGlobal(global); err != nil {
		return Report{}, err
	}
	if strings.TrimSpace(global.ConanBin) != "" {
		a.Client.Binary = global.ConanBin
	}
	if input.Password != "" {
		if err := config.SavePassword(input.Password); err != nil {
			return Report{}, err
		}
	}
	if global.Nexus.Name != "" && global.Nexus.URL != "" {
		if _, addErr := a.RemoteAdd(ctx, global.Nexus.Name, global.Nexus.URL); addErr != nil {
			return Report{OK: false, Action: "config", Message: "全局配置已保存，但添加 Conan remote 失败", Error: addErr.Error(), Data: global.View()}, addErr
		}
		if project, projectErr := a.Project(); projectErr == nil && project.Remote == "" {
			project.Remote = global.Nexus.Name
			_ = config.SaveProject(a.Dir, project)
		}
	}
	return Report{OK: true, Action: "config", Message: "全局设置已保存", Data: global.View()}, nil
}

func (a *App) ConfigLogin(ctx context.Context, password string) (Report, error) {
	global, err := config.LoadGlobal()
	if err != nil {
		return Report{}, err
	}
	if global.Nexus.Name == "" || global.Nexus.Username == "" {
		return Report{}, errors.New("请先在设置中填写仓库名称和用户名")
	}
	if password == "" {
		password, err = config.LoadPassword()
		if err != nil {
			return Report{}, err
		}
	}
	if password == "" {
		password = os.Getenv("CONAN_PASSWORD")
	}
	if password == "" {
		return Report{}, errors.New("缺少密码，请在设置页录入或设置 CONAN_PASSWORD")
	}
	if err := config.SavePassword(password); err != nil {
		return Report{}, err
	}
	if global.Nexus.URL != "" {
		if _, addErr := a.Client.RemoteAdd(ctx, global.Nexus.Name, global.Nexus.URL); addErr != nil {
			// already exists is fine; RemoteAdd uses --force
			_ = addErr
		}
	}
	result, err := a.Client.RemoteLogin(ctx, global.Nexus.Name, global.Nexus.Username, password)
	report := reportFromResult("config-login", result, err)
	if err == nil {
		report.Message = "已登录 " + global.Nexus.Name
		report.Data = global.View()
	}
	return report, err
}

func (a *App) ConfigTest(ctx context.Context) (Report, error) {
	global, err := config.LoadGlobal()
	if err != nil {
		return Report{}, err
	}
	if global.Nexus.Name == "" {
		return Report{}, errors.New("尚未配置全局仓库")
	}
	if global.Nexus.URL != "" {
		if result, addErr := a.Client.RemoteAdd(ctx, global.Nexus.Name, global.Nexus.URL); addErr != nil {
			return reportFromResult("config-test", result, addErr), addErr
		}
	}
	result, listErr := a.Client.Remotes(ctx)
	ok := listErr == nil && remoteListContains(result.Stdout, global.Nexus.Name)
	report := reportFromResult("config-test", result, listErr)
	report.OK = ok
	report.Data = map[string]any{
		"global":  global.View(),
		"present": remoteListContains(result.Stdout, global.Nexus.Name),
	}
	if ok {
		report.Message = "仓库可达: " + global.Nexus.Name
		report.Error = ""
		return report, nil
	}
	report.Message = "测试失败"
	if report.Error == "" {
		report.Error = "Conan remote 列表中没有 " + global.Nexus.Name
	}
	if listErr != nil {
		return report, listErr
	}
	return report, errors.New(report.Error)
}

func hasNamedFile(dir, name string) bool {
	_, err := os.Stat(filepath.Join(dir, name))
	return err == nil
}

func osStatConanfile(dir string) (string, error) {
	for _, name := range []string{"conanfile.py", "conanfile.txt"} {
		if hasNamedFile(dir, name) {
			return name, nil
		}
	}
	return "", errors.New("missing")
}
