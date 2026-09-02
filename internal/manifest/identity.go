package manifest

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type PublishIdentity struct {
	Name        string
	Version     string
	QtVersion   string
	BuildSystem string
}

type IdentityResult struct {
	Path   string `json:"path"`
	Action string `json:"action"`
}

func (r IdentityResult) Hint() string {
	if r.Action == "generate" {
		return "确认发布后会生成或改写 conanfile.py 发布配方，再打包上传"
	}
	return "确认发布后会先把包名和版本写入 conanfile.py，再打包上传"
}

func PlanPublishIdentity(dir string) IdentityResult {
	py := filepath.Join(dir, "conanfile.py")
	result := IdentityResult{Path: py, Action: "generate"}
	if _, err := os.Stat(py); err != nil {
		return result
	}
	if GeneratedKind(dir) == RecipeConsume {
		return result
	}
	result.Action = "patch"
	return result
}

func ApplyPublishIdentity(dir string, ident PublishIdentity) (IdentityResult, error) {
	ident.Name = strings.TrimSpace(ident.Name)
	ident.Version = strings.TrimSpace(ident.Version)
	ident.QtVersion = strings.TrimSpace(ident.QtVersion)
	if err := validateIdentity(ident.Name, ident.Version); err != nil {
		return IdentityResult{}, err
	}
	planned := PlanPublishIdentity(dir)
	if planned.Action == "generate" {
		path, err := Generate(dir, GenerateInput{
			Kind:        RecipePublish,
			Name:        ident.Name,
			Version:     ident.Version,
			QtVersion:   ident.QtVersion,
			BuildSystem: ident.BuildSystem,
			Force:       true,
		})
		if err != nil {
			return IdentityResult{}, err
		}
		planned.Path = path
		return planned, nil
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
	if err := writeAtomic(path, []byte(updated), 0o644); err != nil {
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
