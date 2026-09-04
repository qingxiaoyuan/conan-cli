package manifest

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"conan-cli/internal/atomicfile"
)

type PublishIdentity struct {
	Name        string
	Version     string
	QtVersion   string
	BuildSystem string
	LibDirs     []string
	IncludeDirs []string
}

type IdentityResult struct {
	Path   string `json:"path"`
	Action string `json:"action"`
}

func (r IdentityResult) Hint() string {
	// workspace 组件的配方就在其目录里，就地修改，不提 .conan-cli/recipes/。
	inWorkspace := !strings.Contains(filepath.ToSlash(r.Path), ".conan-cli/recipes/")
	switch r.Action {
	case "generate", "update":
		if inWorkspace {
			return "确认发布后会就地更新该组件自带的发布配方，并打包本机已编译的库（不会再编译）"
		}
		return "确认发布后会写入该组件的发布配方（.conan-cli/recipes/<包名>/），不改仓库根的消费配方，并打包本机已编译的库（不会再编译）"
	case "patch":
		if inWorkspace {
			return "确认发布后会就地更新该组件的手工发布配方，再打包本机已编译的库（不会再编译）"
		}
		return "确认发布后会更新该组件的手工发布配方，再打包本机已编译的库（不会再编译）"
	default:
		return "确认发布后会打包本机已编译的库（不会再编译）"
	}
}

func PlanPublishIdentity(dir, name string) IdentityResult {
	py := PublishRecipePath(dir, name)
	result := IdentityResult{Path: py, Action: "generate"}
	if _, err := os.Stat(py); err != nil {
		return result
	}
	if GeneratedKindAt(py) == "" {
		result.Action = "patch"
		return result
	}
	result.Action = "update"
	return result
}

func ApplyPublishIdentity(dir string, ident PublishIdentity) (IdentityResult, error) {
	ident.Name = strings.TrimSpace(ident.Name)
	ident.Version = strings.TrimSpace(ident.Version)
	ident.QtVersion = strings.TrimSpace(ident.QtVersion)
	if err := validateIdentity(ident.Name, ident.Version); err != nil {
		return IdentityResult{}, err
	}
	planned := PlanPublishIdentity(dir, ident.Name)
	if planned.Action == "patch" {
		if err := patchPublishIdentity(planned.Path, ident); err != nil {
			return IdentityResult{}, err
		}
		return planned, nil
	}
	path, err := Generate(dir, GenerateInput{
		Kind:        RecipePublish,
		Name:        ident.Name,
		Version:     ident.Version,
		QtVersion:   ident.QtVersion,
		BuildSystem: ident.BuildSystem,
		LibDirs:     ident.LibDirs,
		IncludeDirs: ident.IncludeDirs,
		Force:       true,
	})
	if err != nil {
		return IdentityResult{}, err
	}
	planned.Path = path
	if planned.Action == "" {
		planned.Action = "generate"
	}
	return planned, nil
}

// PlanPublishIdentityIn plans the identity action for a workspace component
// whose recipe lives inside dir (e.g. packages/<name>/conanfile.py) instead of
// .conan-cli/recipes/<name>/. Existing recipes are only ever patched in place:
// a hand-written recipe gets a "patch", a conan-cli generated one an "update".
// Without a recipe the caller falls back to the isolated publish recipe flow.
func PlanPublishIdentityIn(dir string) IdentityResult {
	path := filepath.Join(dir, "conanfile.py")
	result := IdentityResult{Path: path, Action: "patch"}
	if GeneratedKindAt(path) != "" {
		result.Action = "update"
	}
	return result
}

// ApplyPublishIdentityIn patches name/version/Qt into the recipe inside dir,
// writing back in place (atomically). Used for workspace components that carry
// their own conanfile.py; nothing is generated or moved.
func ApplyPublishIdentityIn(dir string, ident PublishIdentity) (IdentityResult, error) {
	ident.Name = strings.TrimSpace(ident.Name)
	ident.Version = strings.TrimSpace(ident.Version)
	ident.QtVersion = strings.TrimSpace(ident.QtVersion)
	if err := validateIdentity(ident.Name, ident.Version); err != nil {
		return IdentityResult{}, err
	}
	planned := PlanPublishIdentityIn(dir)
	if _, err := os.Stat(planned.Path); err != nil {
		return IdentityResult{}, fmt.Errorf("workspace 配方不存在: %s", planned.Path)
	}
	if err := patchPublishIdentity(planned.Path, ident); err != nil {
		return IdentityResult{}, err
	}
	return planned, nil
}

func validateIdentity(name, version string) error {
	if name == "" || version == "" {
		return fmt.Errorf("请填写包名和版本号")
	}
	if strings.ContainsAny(name, "/@ \t") {
		return fmt.Errorf("包名不能包含空格或 / @")
	}
	if strings.ContainsAny(version, "/@ \t") {
		return fmt.Errorf("版本号不能包含空格或 / @")
	}
	return nil
}

func patchPublishIdentity(path string, ident PublishIdentity) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read conanfile.py: %w", err)
	}
	updated, err := patchIdentityText(string(data), ident)
	if err != nil {
		return err
	}
	if updated == string(data) {
		return nil
	}
	if err := atomicfile.Write(path, []byte(updated), 0o644); err != nil {
		return fmt.Errorf("write conanfile.py: %w", err)
	}
	return nil
}

func patchIdentityText(text string, ident PublishIdentity) (string, error) {
	updated, err := ensureQuotedAssign(text, "name", ident.Name)
	if err != nil {
		return "", err
	}
	updated, err = ensureQuotedAssign(updated, "version", ident.Version)
	if err != nil {
		return "", err
	}
	return patchQtVersion(updated, ident.QtVersion), nil
}

func ensureQuotedAssign(text, field, value string) (string, error) {
	if next, ok := replaceQuotedAssign(text, field, value); ok {
		return next, nil
	}
	inserted, ok := insertQuotedAssign(text, field, value)
	if !ok {
		return "", fmt.Errorf("conanfile.py 没有静态 %s 字段，也无法插入到 ConanFile 类中", field)
	}
	return inserted, nil
}

func replaceQuotedAssign(text, field, value string) (string, bool) {
	re := regexp.MustCompile(`(?m)^([ \t]+)` + regexp.QuoteMeta(field) + `\s*=\s*(['"])([^'"]*)(['"])`)
	loc := re.FindStringSubmatchIndex(text)
	if loc == nil {
		return text, false
	}
	indent := text[loc[2]:loc[3]]
	quote := text[loc[4]:loc[5]]
	repl := indent + field + " = " + quote + value + quote
	return text[:loc[0]] + repl + text[loc[1]:], true
}

func insertQuotedAssign(text, field, value string) (string, bool) {
	other := "name"
	if field == "name" {
		other = "version"
	}
	otherRe := regexp.MustCompile(`(?m)^([ \t]+)` + other + `\s*=\s*(['"])([^'"]*)(['"])\s*\n`)
	if loc := otherRe.FindStringSubmatchIndex(text); loc != nil {
		indent := text[loc[2]:loc[3]]
		quote := text[loc[4]:loc[5]]
		line := indent + field + " = " + quote + value + quote + "\n"
		at := loc[1]
		if field == "name" {
			at = loc[0]
		}
		return text[:at] + line + text[at:], true
	}
	classRe := regexp.MustCompile(`(?m)^(class[^\n]*ConanFile[^\n]*:\s*\n)`)
	loc := classRe.FindStringSubmatchIndex(text)
	if loc == nil {
		return text, false
	}
	line := "    " + field + " = \"" + value + "\"\n"
	return text[:loc[1]] + line + text[loc[1]:], true
}

func patchQtVersion(text, qt string) string {
	if qt == "" {
		return text
	}
	defaultRe := regexp.MustCompile(`(["']qt_version["']\s*:\s*)(["'])([^"']*)(["'])`)
	if loc := defaultRe.FindStringSubmatchIndex(text); loc != nil {
		repl := text[loc[2]:loc[3]] + text[loc[4]:loc[5]] + qt + text[loc[8]:loc[9]]
		text = text[:loc[0]] + repl + text[loc[1]:]
	}
	listRe := regexp.MustCompile(`(["']qt_version["']\s*:\s*\[)([^\]]*)(\])`)
	loc := listRe.FindStringSubmatchIndex(text)
	if loc == nil {
		return text
	}
	inner := text[loc[4]:loc[5]]
	if strings.Contains(inner, `"`+qt+`"`) || strings.Contains(inner, `'`+qt+`'`) {
		return text
	}
	inner = strings.TrimSpace(inner)
	insert := `"` + qt + `"`
	if inner != "" {
		insert += ", " + inner
	}
	return text[:loc[4]] + insert + text[loc[5]:]
}
