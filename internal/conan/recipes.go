package conan

import (
	"sort"
	"strings"
)

type Recipe struct {
	Name      string `json:"name"`
	Version   string `json:"version"`
	Reference string `json:"reference"`
}

type Package struct {
	Name     string   `json:"name"`
	Versions []string `json:"versions"`
}

func ParseRecipes(data map[string]any) []Recipe {
	seen := map[string]bool{}
	var recipes []Recipe
	var walk func(any)
	walk = func(value any) {
		switch typed := value.(type) {
		case map[string]any:
			for key, child := range typed {
				if key == "error" || key == "revisions" || key == "packages" || key == "info" {
					continue
				}
				if rec, ok := parseRecipeKey(key); ok && !seen[rec.Reference] {
					seen[rec.Reference] = true
					recipes = append(recipes, rec)
				}
				walk(child)
			}
		case []any:
			for _, child := range typed {
				walk(child)
			}
		}
	}
	walk(data)
	sort.Slice(recipes, func(i, j int) bool {
		if recipes[i].Name == recipes[j].Name {
			return recipes[i].Version < recipes[j].Version
		}
		return recipes[i].Name < recipes[j].Name
	})
	return recipes
}

func parseRecipeKey(key string) (Recipe, bool) {
	if strings.Contains(key, ":") {
		return Recipe{}, false
	}
	nameVersion := strings.SplitN(key, "@", 2)[0]
	parts := strings.SplitN(nameVersion, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return Recipe{}, false
	}
	if strings.Contains(parts[0], " ") {
		return Recipe{}, false
	}
	return Recipe{Name: parts[0], Version: parts[1], Reference: parts[0] + "/" + parts[1]}, true
}

func GroupPackages(recipes []Recipe) []Package {
	index := map[string]int{}
	var packages []Package
	for _, recipe := range recipes {
		if i, ok := index[recipe.Name]; ok {
			if !containsString(packages[i].Versions, recipe.Version) {
				packages[i].Versions = append(packages[i].Versions, recipe.Version)
			}
			continue
		}
		index[recipe.Name] = len(packages)
		packages = append(packages, Package{Name: recipe.Name, Versions: []string{recipe.Version}})
	}
	return packages
}

func FilterPackages(packages []Package, query string) []Package {
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" || query == "*" {
		return packages
	}
	query = strings.TrimSuffix(query, "*")
	var out []Package
	for _, pkg := range packages {
		if strings.Contains(strings.ToLower(pkg.Name), query) {
			out = append(out, pkg)
		}
	}
	return out
}

func RemoteListError(data map[string]any) string {
	for _, value := range data {
		child, ok := value.(map[string]any)
		if !ok {
			continue
		}
		if message, ok := child["error"].(string); ok && strings.TrimSpace(message) != "" {
			return message
		}
	}
	return ""
}

func containsString(items []string, wanted string) bool {
	for _, item := range items {
		if item == wanted {
			return true
		}
	}
	return false
}
