package workflow

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"conan-cli/internal/conan"
	"conan-cli/internal/config"
	"conan-cli/internal/manifest"
	"conan-cli/internal/nexus"
	"conan-cli/internal/platform"
	"conan-cli/internal/profile"
)

type Report struct {
	OK       bool    `json:"ok"`
	Action   string  `json:"action"`
	ExitCode int     `json:"exit_code,omitempty"`
	Message  string  `json:"message,omitempty"`
	Error    string  `json:"error,omitempty"`
	Output   string  `json:"output,omitempty"`
	Data     any     `json:"data,omitempty"`
	Checks   []Check `json:"checks,omitempty"`
}

type Check struct {
	Name   string `json:"name"`
	OK     bool   `json:"ok"`
	Detail string `json:"detail,omitempty"`
}

type App struct {
	Dir    string
	Client *conan.Client
}

func New(dir string) *App {
	return newApp(dir, nil)
}

func NewWithOutput(dir string, progress io.Writer) *App {
	return newApp(dir, progress)
}

func newApp(dir string, progress io.Writer) *App {
	abs, err := filepath.Abs(dir)
	if err == nil {
		dir = abs
	}
	client := conan.New(dir)
	if os.Getenv("CONAN_BIN") == "" {
		if global, globalErr := config.LoadGlobal(); globalErr == nil && strings.TrimSpace(global.ConanBin) != "" {
			client.Binary = global.ConanBin
		}
	}
	client.Progress = progress
	return &App{Dir: dir, Client: client}
}

func (a *App) Init(ctx context.Context) (Report, error) {
	_ = ctx
	if err := os.MkdirAll(a.Dir, 0o755); err != nil {
		return Report{}, fmt.Errorf("create project directory: %w", err)
	}
	createdConfig := false
	project := config.NewProject(a.Dir)
	if _, statErr := os.Stat(config.ProjectPath(a.Dir)); statErr == nil {
		loaded, loadErr := config.LoadProject(a.Dir)
		if loadErr != nil {
			return Report{}, loadErr
		}
		project = loaded
	} else {
		createdConfig = true
	}

	createdRecipe := false
	if !manifest.HasConanfile(a.Dir) {
		if _, created, err := manifest.EnsureText(a.Dir, project.BuildSystem); err != nil {
			return Report{}, err
		} else {
			createdRecipe = created
		}
	}

	if len(project.Dependencies) == 0 {
		if deps, depErr := manifest.Dependencies(a.Dir); depErr == nil {
			project.Dependencies = deps
		}
	}

	fillProjectDefaults(a.Dir, project)

	if err := config.SaveProject(a.Dir, project); err != nil {
		return Report{}, err
	}
	if global, globalErr := config.LoadGlobal(); globalErr == nil && project.Remote == "" && global.Nexus.Name != "" {
		project.Remote = global.Nexus.Name
		_ = config.SaveProject(a.Dir, project)
	}

	message := "项目已初始化。请在设置中选择目标平台、Qt 和编译器（与当前开发机无关）"
	if !createdConfig {
		message = "已读取项目配置"
	}
	if createdRecipe {
		message += "，并生成了 conanfile.txt"
	}
	return Report{OK: true, Action: "init", Message: message, Data: project}, nil
}

func fillProjectDefaults(dir string, project *config.Project) {
	_ = dir
	if project == nil {
		return
	}
	if project.Platform.Publish.Empty() {
		project.Platform.Publish = project.Platform.Consume
	}
}

func missingTarget(spec config.PlatformSpec) bool {
	return strings.TrimSpace(spec.OS) == "" || strings.TrimSpace(spec.Arch) == ""
}

func (a *App) Project() (*config.Project, error) {
	return config.LoadProject(a.Dir)
}

func (a *App) Add(dependency string) (Report, error) {
	project, err := a.Project()
	if err != nil {
		return Report{}, err
	}
	if err := config.AddDependency(project, dependency); err != nil {
		return Report{}, err
	}
	originalConfig, err := os.ReadFile(config.ProjectPath(a.Dir))
	if err != nil {
		return Report{}, fmt.Errorf("snapshot project config: %w", err)
	}
	if err := config.SaveProject(a.Dir, project); err != nil {
		return Report{}, err
	}
	manifestPath, err := manifest.Add(a.Dir, dependency)
	if err != nil {
		if restoreErr := os.WriteFile(config.ProjectPath(a.Dir), originalConfig, 0o644); restoreErr != nil {
			return Report{}, fmt.Errorf("add dependency: %w; restore project config: %v", err, restoreErr)
		}
		return Report{}, err
	}
	return Report{OK: true, Action: "add", Message: "dependency added", Data: map[string]any{
		"dependency": dependency,
		"manifest":   manifestPath,
		"project":    project,
	}}, nil
}

func (a *App) ProfileList(ctx context.Context) (Report, error) {
	result, err := (profile.Manager{Client: a.Client}).List(ctx)
	return reportFromResult("profile list", result, err), err
}

func (a *App) RemoteList(ctx context.Context) (Report, error) {
	result, err := (nexus.Manager{Client: a.Client}).List(ctx)
	return reportFromResult("remote list", result, err), err
}

func (a *App) RemoteAdd(ctx context.Context, name, url string) (Report, error) {
	if strings.TrimSpace(name) == "" || strings.TrimSpace(url) == "" {
		return Report{}, errors.New("remote name and URL are required")
	}
	result, err := (nexus.Manager{Client: a.Client}).Add(ctx, name, url)
	if err != nil {
		return reportFromResult("remote add", result, err), err
	}
	data := map[string]string{"name": name, "url": url}
	if project, projectErr := a.Project(); projectErr == nil {
		project.Remote = name
		if saveErr := config.SaveProject(a.Dir, project); saveErr != nil {
			return Report{}, saveErr
		}
		data["project_config"] = config.ProjectPath(a.Dir)
	}
	report := reportFromResult("remote add", result, nil)
	report.Data = data
	return report, nil
}

func (a *App) RemoteLogin(ctx context.Context, name, username, password string) (Report, error) {
	if username == "" || password == "" {
		return Report{}, errors.New("username and password are required")
	}
	result, err := (nexus.Manager{Client: a.Client}).Login(ctx, name, username, password)
	return reportFromResult("remote login", result, err), err
}

func (a *App) Search(ctx context.Context, query, remote string) (Report, error) {
	if remote == "" {
		if project, projectErr := a.Project(); projectErr == nil {
			remote = project.Remote
		}
	}
	data, result, err := a.Client.Search(ctx, query, remote)
	if err != nil {
		return reportFromResult("search", result, err), err
	}
	return Report{OK: true, Action: "search", Data: data}, nil
}

type InstallRequest struct {
	OS           string
	Arch         string
	BuildType    string
	Profile      string
	Remote       string
	OutputFolder string
}

func (a *App) Install(ctx context.Context, profileName, remote, outputFolder string) (Report, error) {
	return a.InstallPlatform(ctx, InstallRequest{Profile: profileName, Remote: remote, OutputFolder: outputFolder})
}

func (a *App) InstallPlatform(ctx context.Context, request InstallRequest) (Report, error) {
	project, err := a.Project()
	if err != nil {
		project, err = a.ensureProject()
		if err != nil {
			return Report{}, err
		}
	}
	if request.Profile == "" {
		request.Profile = project.DefaultProfile
	}
	if request.Remote == "" {
		request.Remote = project.Remote
		if request.Remote == "" {
			if global, globalErr := config.LoadGlobal(); globalErr == nil && global.Nexus.URL != "" {
				request.Remote = global.Nexus.Name
			}
		}
	}
	if request.OutputFolder == "" {
		request.OutputFolder = project.OutputFolder
	}
	if request.OutputFolder == "" {
		request.OutputFolder = "lib"
	}
	project.OutputFolder = request.OutputFolder
	spec := project.Platform.Consume
	if request.OS != "" {
		spec.OS = request.OS
	}
	if request.Arch != "" {
		spec.Arch = request.Arch
	}
	if request.BuildType != "" {
		spec.BuildType = request.BuildType
	}
	if missingTarget(spec) {
		return Report{}, errors.New("请先选择目标操作系统和架构，再拉取 Conan 依赖")
	}
	project.Platform.Consume = spec
	if project.Platform.Publish.Empty() {
		project.Platform.Publish = spec
	}
	_ = config.SaveProject(a.Dir, project)
	settings := platform.Resolve(spec, project.Compiler, project.QtVersion)
	result, err := a.Client.Install(ctx, request.OutputFolder, request.Profile, request.Remote, settings.Args()...)
	report := reportFromResult("install", result, err)
	data := map[string]any{
		"profile":               request.Profile,
		"remote":                request.Remote,
		"output_folder":         request.OutputFolder,
		"missing_binary_policy": project.MissingBinaryPolicy,
		"os":                    settings.OS,
		"arch":                  settings.Arch,
		"build_type":            settings.BuildType,
		"conan_settings":        settings.Map(),
		"qt_version":            project.QtVersion,
		"compiler":              project.Compiler.Display(),
	}
	if err != nil {
		data["hint"] = "仓库中没有匹配该平台的二进制，未执行本机编译。请检查仓库或改目标平台。"
		report.Message = "拉取依赖失败，缺少匹配二进制"
	}
	report.Data = data
	return report, err
}

type PublishRequest struct {
	Name            string
	Version         string
	Ref             string
	Channel         string
	Remote          string
	OS              string
	Arch            string
	BuildType       string
	Compiler        string
	CompilerVersion string
	QtVersion       string
	Profile         string
	Note            string
	DryRun          bool
}

func (a *App) Publish(ctx context.Context, profileName, remote, reference, channel string) (Report, error) {
	return a.PublishPackage(ctx, PublishRequest{Profile: profileName, Remote: remote, Ref: reference, Channel: channel})
}

func (a *App) PublishPackage(ctx context.Context, request PublishRequest) (Report, error) {
	project, err := a.Project()
	if err != nil {
		return Report{}, err
	}
	if request.Profile == "" {
		request.Profile = project.DefaultProfile
	}
	if request.Remote == "" {
		request.Remote = project.Remote
		if request.Remote == "" {
			if global, globalErr := config.LoadGlobal(); globalErr == nil && global.Nexus.URL != "" {
				request.Remote = global.Nexus.Name
			}
		}
	}
	if request.Remote == "" && !request.DryRun {
		return Report{}, errors.New("未配置远程仓库，请先在设置页填写 Nexus 地址")
	}
	if request.Remote == "" {
		request.Remote = ""
	}
	if request.Channel == "" {
		request.Channel = project.Channel
	}
	request.Name = strings.TrimSpace(request.Name)
	request.Version = strings.TrimSpace(request.Version)
	reference := strings.TrimSpace(request.Ref)
	if (request.Name == "" || request.Version == "") && reference != "" {
		name, version := splitNameVersion(reference)
		if request.Name == "" {
			request.Name = name
		}
		if request.Version == "" {
			request.Version = version
		}
	}
	if request.Name == "" || request.Version == "" {
		metadata, _, inspectErr := a.Client.Inspect(ctx)
		if inspectErr != nil {
			return Report{}, fmt.Errorf("请填写包名和版本，或先生成发布配方: %w", inspectErr)
		}
		name, _ := metadata["name"].(string)
		version, _ := metadata["version"].(string)
		if request.Name == "" {
			request.Name = strings.TrimSpace(name)
		}
		if request.Version == "" {
			request.Version = strings.TrimSpace(version)
		}
	}
	if err := validatePublishIdentity(request.Name, request.Version); err != nil {
		return Report{}, err
	}
	reference = request.Name + "/" + request.Version
	spec := project.Platform.Publish
	if request.OS != "" {
		spec.OS = request.OS
	}
	if request.Arch != "" {
		spec.Arch = request.Arch
	}
	if request.BuildType != "" {
		spec.BuildType = request.BuildType
	}
	if missingTarget(spec) {
		return Report{}, errors.New("请在发布页选择目标操作系统和架构")
	}
	compiler := project.Compiler
	if request.Compiler != "" {
		compiler.ID = request.Compiler
	}
	if request.CompilerVersion != "" {
		compiler.Version = request.CompilerVersion
	}
	qt := strings.TrimSpace(request.QtVersion)
	if qt == "" {
		qt = project.QtVersion
	}
	if strings.TrimSpace(compiler.ID) == "" || strings.TrimSpace(compiler.Version) == "" {
		return Report{}, errors.New("请在发布页填写编译器类型和版本，例如 gcc 13")
	}
	if qt == "" {
		return Report{}, errors.New("请在发布页填写 Qt 版本，例如 6.8")
	}
	settings := platform.Resolve(spec, compiler, qt)
	recipePlan := manifest.PlanPublishIdentity(a.Dir)
	summary := map[string]any{
		"reference":      reference,
		"name":           request.Name,
		"version":        request.Version,
		"profile":        request.Profile,
		"remote":         request.Remote,
		"channel":        request.Channel,
		"os":             settings.OS,
		"arch":           settings.Arch,
		"build_type":     settings.BuildType,
		"compiler":       compiler.Display(),
		"qt_version":     qt,
		"conan_settings": settings.Map(),
		"note":           request.Note,
		"recipe_action":  recipePlan.Action,
		"recipe_path":    recipePlan.Path,
		"recipe_hint":    recipePlan.Hint(),
		"command":        fmt.Sprintf("conan-cli publish --remote %s --channel %s --os %s --arch %s --build-type %s --compiler %s --compiler-version %s --qt %s --name %s --version %s", request.Remote, request.Channel, settings.OS, settings.Arch, settings.BuildType, compiler.ID, compiler.Version, qt, request.Name, request.Version),
	}
	if request.DryRun {
		return Report{OK: true, Action: "publish-preview", Message: recipePlan.Hint(), Data: summary}, nil
	}
	applied, err := manifest.ApplyPublishIdentity(a.Dir, manifest.PublishIdentity{
		Name:        request.Name,
		Version:     request.Version,
		QtVersion:   qt,
		BuildSystem: project.BuildSystem,
	})
	if err != nil {
		return Report{OK: false, Action: "publish", Error: err.Error(), Data: summary}, err
	}
	summary["recipe_action"] = applied.Action
	summary["recipe_path"] = applied.Path
	if _, saveErr := a.SaveProjectSettings(ProjectSettingsInput{
		Name: request.Name, QtVersion: qt, CompilerID: compiler.ID, CompilerVersion: compiler.Version,
		PublishOS: spec.OS, PublishArch: spec.Arch, PublishBuildType: spec.BuildType, Channel: request.Channel,
	}); saveErr != nil {
		return Report{OK: false, Action: "publish", Error: saveErr.Error(), Data: summary}, saveErr
	}
	createUser, createChannel := referenceCoordinates(request.Ref, request.Channel)
	if createUser == "" {
		createUser, createChannel = referenceCoordinates(reference, request.Channel)
	}
	create, err := a.Client.Create(ctx, request.Profile, createUser, createChannel, settings.Args()...)
	if err != nil {
		return reportFromResult("publish", create, err), err
	}
	upload, err := a.Client.Upload(ctx, reference, request.Remote)
	if err != nil {
		return reportFromResult("publish", upload, err), err
	}
	return Report{OK: true, Action: "publish", Message: "已更新 conanfile.py，包已创建并上传", Output: joinOutput(resultOutput(create), resultOutput(upload)), Data: summary}, nil
}

func validatePublishIdentity(name, version string) error {
	if strings.TrimSpace(name) == "" || strings.TrimSpace(version) == "" {
		return errors.New("请在发布页填写包名和版本号")
	}
	if strings.ContainsAny(name, "/@ \t") {
		return errors.New("包名不能包含空格或 / @")
	}
	if strings.ContainsAny(version, "/@ \t") {
		return errors.New("版本号不能包含空格或 / @")
	}
	return nil
}

func (a *App) Doctor(ctx context.Context) (Report, error) {
	checks := []Check{}
	version, versionErr := a.Client.Version(ctx)
	checks = append(checks, Check{Name: "conan", OK: versionErr == nil, Detail: firstLine(version.Stdout, version.Stderr)})
	project, projectErr := a.Project()
	checks = append(checks, Check{Name: "project_config", OK: projectErr == nil, Detail: config.ProjectPath(a.Dir)})
	hasRecipe := hasConanfile(a.Dir)
	recipeDetail := "缺少 conanfile.py / conanfile.txt"
	if hasRecipe {
		if fileExists(a.Dir, "conanfile.py") {
			recipeDetail = "conanfile.py"
		} else {
			recipeDetail = "conanfile.txt"
		}
	}
	checks = append(checks, Check{Name: "conanfile", OK: hasRecipe, Detail: recipeDetail})
	if projectErr == nil && hasConanfile(a.Dir) {
		dependencies, dependenciesErr := manifest.Dependencies(a.Dir)
		if errors.Is(dependenciesErr, manifest.ErrDynamicRequirements) {
			checks = append(checks, Check{Name: "manifest_dependencies", OK: true, Detail: "动态 requirements()，已跳过对比"})
		} else if dependenciesErr != nil {
			checks = append(checks, Check{Name: "manifest_dependencies", OK: false, Detail: dependenciesErr.Error()})
		} else {
			checks = append(checks, dependencyCheck(project.Dependencies, dependencies))
		}
	}
	profiles, profilesErr := a.Client.Profiles(ctx)
	checks = append(checks, Check{Name: "profiles", OK: profilesErr == nil, Detail: firstLine(profiles.Stdout, profiles.Stderr)})
	remotes, remotesErr := a.Client.Remotes(ctx)
	checks = append(checks, Check{Name: "remotes", OK: remotesErr == nil, Detail: firstLine(remotes.Stdout, remotes.Stderr)})
	if global, globalErr := config.LoadGlobal(); globalErr == nil && global.Nexus.Name != "" {
		password, _ := config.LoadPassword()
		detail := global.Nexus.Name
		if global.Nexus.Username != "" {
			detail += " / " + global.Nexus.Username
		}
		if password == "" {
			detail += "（未保存密码）"
		}
		checks = append(checks, Check{Name: "global_remote", OK: global.Nexus.URL != "" && global.Nexus.Username != "" && password != "", Detail: detail})
		if projectErr == nil && project.Remote == "" {
			project.Remote = global.Nexus.Name
		}
	}
	if projectErr == nil && project.Remote != "" {
		checks = append(checks, Check{Name: "configured_remote", OK: remoteListContains(remotes.Stdout, project.Remote), Detail: project.Remote})
	}
	if projectErr == nil {
		platformOK := !missingTarget(project.Platform.Consume)
		detail := project.Platform.Consume.Display()
		if !platformOK {
			detail = "未选择"
		}
		checks = append(checks, Check{Name: "platform", OK: platformOK, Detail: detail})
	}

	ok := true
	for _, check := range checks {
		ok = ok && check.OK
	}
	return Report{OK: ok, Action: "doctor", Checks: checks}, nil
}

func dependencyCheck(projectDependencies, manifestDependencies []string) Check {
	projectSet := make(map[string]struct{}, len(projectDependencies))
	manifestSet := make(map[string]struct{}, len(manifestDependencies))
	for _, dependency := range projectDependencies {
		projectSet[dependency] = struct{}{}
	}
	for _, dependency := range manifestDependencies {
		manifestSet[dependency] = struct{}{}
	}
	if len(projectSet) != len(manifestSet) {
		return Check{Name: "manifest_dependencies", OK: false, Detail: "项目配置与配方依赖不一致"}
	}
	for dependency := range projectSet {
		if _, ok := manifestSet[dependency]; !ok {
			return Check{Name: "manifest_dependencies", OK: false, Detail: "项目配置与配方依赖不一致"}
		}
	}
	return Check{Name: "manifest_dependencies", OK: true, Detail: "项目配置与配方依赖一致"}
}

func reportFromResult(action string, result conan.Result, err error) Report {
	report := Report{OK: err == nil, Action: action, Output: resultOutput(result)}
	if err != nil {
		report.ExitCode = resultExitCode(result)
		report.Message = err.Error()
		report.Error = err.Error()
		if report.Output == "" && !result.Streamed {
			report.Output = strings.TrimSpace(result.Stderr)
		}
	}
	return report
}

func resultOutput(result conan.Result) string {
	if result.Streamed {
		return ""
	}
	return strings.TrimSpace(result.Stdout)
}

func resultExitCode(result conan.Result) int {
	if result.Code > 0 {
		return result.Code
	}
	return 1
}

func splitNameVersion(reference string) (string, string) {
	reference = strings.TrimSpace(reference)
	if i := strings.Index(reference, "@"); i >= 0 {
		reference = reference[:i]
	}
	parts := strings.SplitN(reference, "/", 2)
	if len(parts) != 2 {
		return "", ""
	}
	return strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
}

func referenceCoordinates(reference, fallbackChannel string) (string, string) {
	channel := fallbackChannel
	user := ""
	parts := strings.SplitN(reference, "@", 2)
	if len(parts) != 2 {
		return user, channel
	}
	coordinates := strings.SplitN(parts[1], "/", 2)
	if len(coordinates) == 2 {
		user = coordinates[0]
		channel = coordinates[1]
	}
	return user, channel
}

func remoteListContains(output, wanted string) bool {
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Fields(strings.TrimSpace(line))
		if len(fields) == 0 {
			continue
		}
		name := strings.TrimSuffix(fields[0], ":")
		if name == wanted {
			return true
		}
	}
	return false
}

func hasConanfile(dir string) bool {
	for _, name := range []string{"conanfile.py", "conanfile.txt"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err == nil {
			return true
		}
	}
	return false
}

func firstLine(values ...string) string {
	for _, value := range values {
		for _, line := range strings.Split(strings.TrimSpace(value), "\n") {
			if strings.TrimSpace(line) != "" {
				return strings.TrimSpace(line)
			}
		}
	}
	return ""
}

func joinOutput(values ...string) string {
	var nonEmpty []string
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			nonEmpty = append(nonEmpty, strings.TrimSpace(value))
		}
	}
	return strings.Join(nonEmpty, "\n")
}
