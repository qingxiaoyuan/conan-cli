package conan

import "testing"

func TestExtractBinaries(t *testing.T) {
	data := map[string]any{
		"nexus": map[string]any{
			"fmt/10.2.1": map[string]any{
				"revisions": map[string]any{
					"abc": map[string]any{
						"packages": map[string]any{
							"pkgid": map[string]any{
								"info": map[string]any{
									"settings": map[string]any{"os": "Linux", "arch": "x86_64", "compiler": "gcc", "compiler.version": "11"},
								},
							},
						},
					},
				},
			},
		},
	}
	binaries := ExtractBinaries(data)
	if len(binaries) != 1 || binaries[0].Settings["os"] != "Linux" {
		t.Fatalf("binaries = %#v", binaries)
	}
	if !ListHasReference(data, "fmt/10.2.1") {
		t.Fatal("expected reference to be found")
	}
}
