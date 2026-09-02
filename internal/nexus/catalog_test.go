package nexus

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestParseRepositoryURL(t *testing.T) {
	base, repo, ok := ParseRepositoryURL("http://172.20.89.163:29081/repository/conan-hosted/")
	if !ok || base != "http://172.20.89.163:29081" || repo != "conan-hosted" {
		t.Fatalf("base=%q repo=%q ok=%v", base, repo, ok)
	}
	if _, _, ok := ParseRepositoryURL("https://center.conan.io"); ok {
		t.Fatal("expected non-nexus URL to fail")
	}
}

func TestCleanConanVersion(t *testing.T) {
	if got := CleanConanVersion("1.0-_#b70eb684b5a9a2f26257796bbf31c0e4"); got != "1.0" {
		t.Fatalf("got %q", got)
	}
	if got := CleanConanVersion("1.2.3"); got != "1.2.3" {
		t.Fatalf("got %q", got)
	}
}

func TestListPackagesFromNexus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/service/rest/v1/components" || r.URL.Query().Get("repository") != "conan-hosted" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"items":[{"name":"qtutils","version":"1.0-_#abc","format":"conan"},{"name":"qtutils","version":"1.1-_#def","format":"conan"},{"name":"fmt","version":"10.2.1-_#x","format":"conan"}]}`))
	}))
	defer server.Close()
	packages, err := ListPackages(context.Background(), server.URL+"/repository/conan-hosted/", "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(packages) != 2 || packages[0].Name != "fmt" || len(packages[1].Versions) != 2 {
		t.Fatalf("packages = %#v", packages)
	}
}
