package workflow

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"conan-cli/internal/atomicfile"
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
			client.UseExecutable(global.ConanBin)
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
	if global, globalErr := config.LoadGlobal(); globalErr == nil && project.Remote == "" && global.Nexus.Name != "" {
		project.Remote = global.Nexus.Name
	}
	if err := config.SaveProject(a.Dir, project); err != nil {
		return Report{}, err
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

// resolveRemote is the single remote fallback rule shared by every command:
// 显式参数 → project.Remote → 全局 Nexus 名。全局仓库只有在名称和 URL 都
// 配置齐全时才作为兜底，避免把一个没有 URL、实际未注册的 remote 传给
// Conan。集中在这里保证 CLI / TUI / VS Code 三个入口行为一致。
func resolveRemote(explicit string, project *config.Project) string {
	explicit = strings.TrimSpace(explicit)
	if explicit != "" {
		return explicit
	}
	if project != nil && strings.TrimSpace(project.Remote) != "" {
		return strings.TrimSpace(project.Remote)
	}
	if global, err := config.LoadGlobal(); err == nil && global.Nexus.URL != "" && global.Nexus.Name != "" {
		return global.Nexus.Name
	}
	return ""
}

func fillProjectDefaults(dir string, project *config.Project) {
	if project == nil {
		return
	}
	// 只做包名探测；consume → publish 的平台回退统一由 config.applyDefaults
	// 在每次 SaveProject 时处理，避免多处复制同一规则。
	applyPackageIdentity(dir, project)
}

func applyPackageIdentity(dir string, project *config.Project) bool {
	if project == nil || project.NameLocked && strings.TrimSpace(project.Name) != "" {
		return false
	}
	guess := manifest.DetectPackageName(dir)
	if guess.Name == "" || guess.Name == project.Name {
		return false
	}
	project.Name = guess.Name
	return true
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
		if restoreErr := atomicfile.Write(config.ProjectPath(a.Dir), originalConfig, 0o644); restoreErr != nil {
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
	request.Remote = resolveRemote(request.Remote, project)
	if request.OutputFolder == "" {
		request.OutputFolder = project.OutputFolder
	}
	if request.OutputFolder == "" {
		request.OutputFolder = config.DefaultOutputFolder
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
	// 保存失败不阻塞下载本身，但不能静默吞掉：把它带进报告数据。
	warning := ""
	if saveErr := config.SaveProject(a.Dir, project); saveErr != nil {
		warning = "项目配置保存失败（不影响本次下载）：" + saveErr.Error()
	}
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
	if warning != "" {
		data["warning"] = warning
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

// publishPlan is a fully resolved publish target: identity, remote, platform,
// toolchain, and the Conan settings derived from them. resolvePublishPlan
// produces it so PublishPackage stays a short orchestration.
type publishPlan struct {
	Name      string
	Version   string
	Reference string
	Channel   string
	Remote    string
	Profile   string
	Note      string
	Spec      config.PlatformSpec
	Compiler  config.Compiler
	QtVersion string
	Settings  platform.Settings
}

func (a *App) Publish(ctx context.Context, profileName, remote, reference, channel string) (Report, error) {
	return a.PublishPackage(ctx, PublishRequest{Profile: profileName, Remote: remote, Ref: reference, Channel: channel})
}

func (a *App) PublishPackage(ctx context.Context, request PublishRequest) (Report, error) {
	project, err := a.Project()
	if err != nil {
		return Report{}, err
	}
	plan, err := a.resolvePublishPlan(ctx, project, request)
	if err != nil {
		return Report{}, err
	}
	if !request.DryRun && !manifest.HasPrebuiltLibraries(a.Dir) {
		return Report{}, errors.New("未找到已编译的库（例如 lib/*.so、lib/*.a、lib/*.dll）。请先用发布页同一套系统/编译器/Qt/Debug|Release 在本机编好，再发布")
	}
	recipePlan := manifest.PlanPublishIdentity(a.Dir)
	summary := plan.summary(recipePlan)
	if request.DryRun {
		return Report{OK: true, Action: "publish-preview", Message: recipePlan.Hint(), Data: summary}, nil
	}
	applied, err := a.applyPublishRecipe(plan, project.BuildSystem)
	if err != nil {
		return Report{OK: false, Action: "publish", Error: err.Error(), Data: summary}, err
	}
	summary["recipe_action"] = applied.Action
	summary["recipe_path"] = applied.Path
	report, err := a.uploadPackage(ctx, plan)
	report.Data = summary
	return report, err
}

// resolvePublishPlan fills every publish default: profile, remote (via the
// shared resolveRemote rule), package identity (request → ref → project →
// recipe inspect), publish platform, and toolchain. It may lock and persist a
// renamed package identity as a side effect.
func (a *App) resolvePublishPlan(ctx context.Context, project *config.Project, request PublishRequest) (*publishPlan, error) {
	plan := &publishPlan{
		Channel: strings.TrimSpace(request.Channel),
		Note:    request.Note,
		Profile: request.Profile,
	}
	if plan.Profile == "" {
		plan.Profile = project.DefaultProfile
	}
	plan.Remote = resolveRemote(request.Remote, project)
	if plan.Remote == "" && !request.DryRun {
		return nil, errors.New("未配置远程仓库，请先在设置页填写 Nexus 地址")
	}

	if applyPackageIdentity(a.Dir, project) {
		if err := config.SaveProject(a.Dir, project); err != nil {
			return nil, fmt.Errorf("保存项目配置失败: %w", err)
		}
	}
	request.Name = strings.TrimSpace(request.Name)
	request.Version = strings.TrimSpace(request.Version)
	if reference := strings.TrimSpace(request.Ref); reference != "" && (request.Name == "" || request.Version == "") {
		name, version := splitNameVersion(reference)
		if request.Name == "" {
			request.Name = name
		}
		if request.Version == "" {
			request.Version = version
		}
	}
	identity := strings.TrimSpace(project.Name)
	if request.Name == "" {
		request.Name = identity
	} else if identity != "" && request.Name != identity {
		project.Name = request.Name
		project.NameLocked = true
		if err := config.SaveProject(a.Dir, project); err != nil {
			return nil, fmt.Errorf("保存项目配置失败: %w", err)
		}
	}
	if request.Version == "" {
		if metadata, _, inspectErr := a.Client.Inspect(ctx); inspectErr == nil {
			version, _ := metadata["version"].(string)
			request.Version = strings.TrimSpace(version)
		}
	}
	if err := validatePublishIdentity(request.Name, request.Version); err != nil {
		return nil, err
	}
	plan.Name, plan.Version = request.Name, request.Version
	plan.Reference = request.Name + "/" + request.Version

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
		return nil, errors.New("请在发布页选择目标操作系统和架构")
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
		return nil, errors.New("请在发布页填写编译器类型和版本，例如 gcc 13")
	}
	if qt == "" {
		return nil, errors.New("请在发布页填写 Qt 版本，例如 6.8")
	}
	plan.Spec, plan.Compiler, plan.QtVersion = spec, compiler, qt
	plan.Settings = platform.Resolve(spec, compiler, qt)
	return plan, nil
}

// summary renders the data payload shared by dry-run preview and the real
// publish report.
func (p *publishPlan) summary(recipePlan manifest.IdentityResult) map[string]any {
	return map[string]any{
		"reference":      p.Reference,
		"name":           p.Name,
		"version":        p.Version,
		"channel":        p.Channel,
		"profile":        p.Profile,
		"remote":         p.Remote,
		"os":             p.Settings.OS,
		"arch":           p.Settings.Arch,
		"build_type":     p.Settings.BuildType,
		"compiler":       p.Compiler.Display(),
		"qt_version":     p.QtVersion,
		"conan_settings": p.Settings.Map(),
		"note":           p.Note,
		"recipe_action":  recipePlan.Action,
		"recipe_path":    recipePlan.Path,
		"recipe_hint":    recipePlan.Hint(),
		"command": fmt.Sprintf("conan-cli publish --remote %s --os %s --arch %s --build-type %s --compiler %s --compiler-version %s --qt %s --name %s --version %s",
			p.Remote, p.Settings.OS, p.Settings.Arch, p.Settings.BuildType, p.Compiler.ID, p.Compiler.Version, p.QtVersion, p.Name, p.Version),
	}
}

// applyPublishRecipe patches name/version/Qt into the recipe and persists the
// publish toolchain so the next publish starts from the same target.
func (a *App) applyPublishRecipe(plan *publishPlan, buildSystem string) (manifest.IdentityResult, error) {
	applied, err := manifest.ApplyPublishIdentity(a.Dir, manifest.PublishIdentity{
		Name:        plan.Name,
		Version:     plan.Version,
		QtVersion:   plan.QtVersion,
		BuildSystem: buildSystem,
	})
	if err != nil {
		return applied, err
	}
	if _, err := a.SaveProjectSettings(ProjectSettingsInput{
		QtVersion: plan.QtVersion, CompilerID: plan.Compiler.ID, CompilerVersion: plan.Compiler.Version,
		PublishOS: plan.Spec.OS, PublishArch: plan.Spec.Arch, PublishBuildType: plan.Spec.BuildType,
	}); err != nil {
		return applied, err
	}
	return applied, nil
}

// uploadPackage packs the prebuilt libraries with export-pkg and uploads the
// reference to the remote.
func (a *App) uploadPackage(ctx context.Context, plan *publishPlan) (Report, error) {
	packed, err := a.Client.ExportPkg(ctx, plan.Profile, plan.Settings.Args()...)
	if err != nil {
		return reportFromResult("publish", packed, err), err
	}
	upload, err := a.Client.Upload(ctx, plan.Reference, plan.Remote)
	if err != nil {
		return reportFromResult("publish", upload, err), err
	}
	return Report{OK: true, Action: "publish", Message: "已打包本机预编译库并上传", Output: joinOutput(resultOutput(packed), resultOutput(upload))}, nil
}

func validatePublishIdentity(name, version string) error {
	if strings.TrimSpace(name) == "" {
		return errors.New("无法确定包名。请在设置中填写 Conan 包名")
	}
	if strings.TrimSpace(version) == "" {
		return errors.New("请在发布页填写版本号")
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
	conanDetail := firstLine(version.Stdout, version.Stderr)
	if versionErr == nil && len(a.Client.BaseArgs) > 0 {
		conanDetail = strings.TrimSpace(conanDetail + "（插件内置 Python）")
	}
	checks = append(checks, Check{Name: "conan", OK: versionErr == nil, Detail: conanDetail})
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
