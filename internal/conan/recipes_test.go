package conan

import "testing"

func TestParseAndGroupRecipes(t *testing.T) {
	data := map[string]any{
		"nexus": map[string]any{
			"qtutils/1.0": map[string]any{},
			"qtutils/1.1": map[string]any{},
			"fmt/10.2.1":  map[string]any{},
			"error":       "ignore me",
		},
	}
	recipes := ParseRecipes(data)
	if len(recipes) != 3 {
		t.Fatalf("recipes = %#v", recipes)
	}
	packages := GroupPackages(recipes)
	if len(packages) != 2 {
		t.Fatalf("packages = %#v", packages)
	}
	filtered := FilterPackages(packages, "qt")
	if len(filtered) != 1 || filtered[0].Name != "qtutils" || len(filtered[0].Versions) != 2 {
		t.Fatalf("filtered = %#v", filtered)
	}
}
