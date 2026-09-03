package workflow

import (
	"strings"

	"conan-cli/internal/config"
	"conan-cli/internal/manifest"
)

func (a *App) GenerateRecipe(kind string, force bool, name, version, qt string) (Report, error) {
	project, err := a.ensureProject()
	if err != nil {
		return Report{}, err
	}
	if applyPackageIdentity(a.Dir, project) {
		_ = config.SaveProject(a.Dir, project)
	}
	name = strings.TrimSpace(name)
	if name != "" && name != project.Name {
		project.Name = name
		project.NameLocked = true
		_ = config.SaveProject(a.Dir, project)
	}
	input := manifest.GenerateInput{
		Kind:        manifest.RecipeKind(strings.TrimSpace(kind)),
		Name:        firstNonEmpty(name, project.Name),
		Version:     firstNonEmpty(version, "1.0"),
		BuildSystem: project.BuildSystem,
		QtVersion:   firstNonEmpty(qt, project.QtVersion),
		Requires:    project.Dependencies,
		Force:       force,
	}
	path, err := manifest.Generate(a.Dir, input)
	if err != nil {
		return Report{OK: false, Action: "recipe-generate", Error: err.Error()}, err
	}
	label := "消费配方"
	if input.Kind == manifest.RecipePublish {
		label = "发布配方"
	}
	return Report{
		OK:      true,
		Action:  "recipe-generate",
		Message: "已生成" + label + " " + path,
		Data: map[string]any{
			"path": path,
			"kind": string(input.Kind),
			"name": input.Name,
		},
	}, nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
