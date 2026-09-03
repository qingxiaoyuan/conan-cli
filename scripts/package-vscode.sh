#!/usr/bin/env bash
# Rebuild CLI binaries, bundle portable Python + Conan 2, and build the VS Code VSIX.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
ONLY="${1:-all}"

build_cli() {
  local goos="$1" goarch="$2" dest="$3"
  mkdir -p "$(dirname "$dest")"
  echo "编译 $goos/$goarch -> $dest"
  CGO_ENABLED=0 GOOS="$goos" GOARCH="$goarch" go build -trimpath -ldflags='-s -w' -o "$dest" "$ROOT/cmd/conan-cli"
}

build_cli linux amd64 "$ROOT/vscode/bin/linux-x64/conan-cli"
build_cli linux arm64 "$ROOT/vscode/bin/linux-arm64/conan-cli"
build_cli darwin amd64 "$ROOT/vscode/bin/darwin-x64/conan-cli"
build_cli darwin arm64 "$ROOT/vscode/bin/darwin-arm64/conan-cli"
build_cli windows amd64 "$ROOT/vscode/bin/win32-x64/conan-cli.exe"
mkdir -p "$ROOT/dist"
cp -f "$ROOT/vscode/bin/linux-x64/conan-cli" "$ROOT/dist/conan-cli-linux-amd64"

"$ROOT/scripts/bundle-runtime.sh" "$ONLY"

version="$(python3 -c "import json; print(json.load(open('$ROOT/vscode/package.json'))['version'])")"
out="$ROOT/dist/conan-cli-vscode-${version}.vsix"
python3 - "$ROOT/vscode" "$out" "$version" <<'PY'
import json, os, sys, zipfile
from pathlib import Path

src, dest, version = Path(sys.argv[1]), Path(sys.argv[2]), sys.argv[3]
pkg = json.loads((src / "package.json").read_text())
manifest = f"""<?xml version="1.0" encoding="utf-8"?>
<PackageManifest Version="2.0.0" xmlns="http://schemas.microsoft.com/developer/vsx-schema/2011" xmlns:d="http://schemas.microsoft.com/developer/vsx-schema-design/2011">
<Metadata>
<Identity Language="en-US" Id="{pkg["name"]}" Version="{version}" Publisher="{pkg.get("publisher", "local")}" />
<DisplayName>{pkg.get("displayName", pkg["name"])}</DisplayName>
<Description xml:space="preserve">{pkg.get("description", "")}</Description>
<Tags></Tags>
<Categories>Other</Categories>
<GalleryFlags>Public</GalleryFlags>
<Properties>
<Property Id="Microsoft.VisualStudio.Code.Engine" Value="{pkg.get("engines", {}).get("vscode", "^1.85.0")}" />
<Property Id="Microsoft.VisualStudio.Code.ExtensionDependencies" Value="" />
<Property Id="Microsoft.VisualStudio.Code.ExtensionPack" Value="" />
<Property Id="Microsoft.VisualStudio.Code.ExtensionKind" Value="workspace" />
<Property Id="Microsoft.VisualStudio.Code.LocalizedLanguages" Value="" />
<Property Id="Microsoft.VisualStudio.Code.EnabledApiProposals" Value="" />
<Property Id="Microsoft.VisualStudio.Code.ExecutesCode" Value="true" />
<Property Id="Microsoft.VisualStudio.Services.GitHubFlavoredMarkdown" Value="true" />
<Property Id="Microsoft.VisualStudio.Services.Content.Pricing" Value="Free"/>
</Properties>
<Icon>extension/media/conan.png</Icon>
</Metadata>
<Installation>
<InstallationTarget Id="Microsoft.VisualStudio.Code"/>
</Installation>
<Dependencies/>
<Assets>
<Asset Type="Microsoft.VisualStudio.Code.Manifest" Path="extension/package.json" Addressable="true" />
<Asset Type="Microsoft.VisualStudio.Services.Content.Details" Path="extension/readme.md" Addressable="true" />
<Asset Type="Microsoft.VisualStudio.Services.Icons.Default" Path="extension/media/conan.png" Addressable="true" />
</Assets>
</PackageManifest>
"""
content_types = """<?xml version="1.0" encoding="utf-8"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
<Default Extension=".dll" ContentType="application/octet-stream"/>
<Default Extension=".dylib" ContentType="application/octet-stream"/>
<Default Extension=".exe" ContentType="application/octet-stream"/>
<Default Extension=".html" ContentType="text/html"/>
<Default Extension=".js" ContentType="application/javascript"/>
<Default Extension=".json" ContentType="application/json"/>
<Default Extension=".md" ContentType="text/markdown"/>
<Default Extension=".png" ContentType="image/png"/>
<Default Extension=".py" ContentType="text/x-python"/>
<Default Extension=".so" ContentType="application/octet-stream"/>
<Default Extension=".svg" ContentType="image/svg+xml"/>
<Default Extension=".txt" ContentType="text/plain"/>
<Default Extension=".vsixmanifest" ContentType="text/xml"/>
</Types>
"""
skip_names = {".gitignore", ".vscodeignore"}
skip_suffixes = {".vsix", ".tar.gz", ".pyc", ".pyo"}
skip_dirs = {".vscode", "node_modules", "__pycache__"}

def include(path: Path) -> bool:
    rel = path.relative_to(src)
    if any(part in skip_dirs for part in rel.parts):
        return False
    if path.name in skip_names or path.name.startswith("."):
        return False
    if path.suffix in skip_suffixes:
        return False
    return True

dest.parent.mkdir(parents=True, exist_ok=True)
count = 0
with zipfile.ZipFile(dest, "w", compression=zipfile.ZIP_DEFLATED, compresslevel=6) as zf:
    zf.writestr("extension.vsixmanifest", manifest)
    zf.writestr("[Content_Types].xml", content_types)
    for path in sorted(src.rglob("*")):
        if not path.is_file() or not include(path):
            continue
        rel = path.relative_to(src).as_posix()
        if rel == "README.md":
            rel = "readme.md"
        zf.write(path, "extension/" + rel)
        count += 1
print(f"packed {count} files -> {dest} ({dest.stat().st_size} bytes)")
PY
ls -lh "$out"
echo "已生成 $out"
