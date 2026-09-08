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

func TestExtractBinaryRefs(t *testing.T) {
	data := map[string]any{
		"nexus": map[string]any{
			"qtutils/1.0@team/dev": map[string]any{
				"revisions": map[string]any{
					"abc": map[string]any{
						"packages": map[string]any{
							"pkgid": map[string]any{
								"info": map[string]any{
									"settings": map[string]any{
										"os": "Linux", "arch": "x86_64", "compiler": "gcc",
										"compiler.version": "11", "build_type": "Release",
									},
									"options": map[string]any{"qt_version": "6.8"},
								},
							},
						},
					},
				},
			},
		},
	}
	refs := ExtractBinaryRefs(data)
	if len(refs) != 1 {
		t.Fatalf("refs = %#v", refs)
	}
	if refs[0].Name != "qtutils" || refs[0].Version != "1.0" || refs[0].Channel != "dev" {
		t.Fatalf("identity = %#v", refs[0])
	}
	if refs[0].Settings["arch"] != "x86_64" || refs[0].Options["qt_version"] != "6.8" {
		t.Fatalf("payload = %#v", refs[0])
	}
}
