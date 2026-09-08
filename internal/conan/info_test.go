package conan

import "testing"

func TestParseConanInfo(t *testing.T) {
	settings, options := ParseConanInfo(`[settings]
    arch=x86_64
    build_type=Release
    compiler=gcc
    compiler.version=11
    os=Linux
[options]
    fPIC=True
    qt_version=6.8
    shared=True
[requires]
    zlib/1.2.13
`)
	if settings["os"] != "Linux" || settings["arch"] != "x86_64" || settings["compiler"] != "gcc" || settings["build_type"] != "Release" {
		t.Fatalf("settings = %#v", settings)
	}
	if options["qt_version"] != "6.8" {
		t.Fatalf("options = %#v", options)
	}
}

func TestParseConanInfoFullSettingsFallback(t *testing.T) {
	settings, _ := ParseConanInfo("[full_settings]\nos=Windows\narch=x86\n")
	if settings["os"] != "Windows" || settings["arch"] != "x86" {
		t.Fatalf("settings = %#v", settings)
	}
}
