#!/usr/bin/env bash
# Download relocatable CPython and install Conan 2 into vscode/runtime/<platform>/.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
PYTHON_BUILD="${PYTHON_BUILD:-20260901}"
PYTHON_VERSION="${PYTHON_VERSION:-3.12.14}"
PYTHON_XY="${PYTHON_VERSION%.*}"
CONAN_VERSION="${CONAN_VERSION:-2.32.0}"
CACHE="${CACHE_DIR:-$ROOT/.cache/python-standalone}"
DEST="${DEST_DIR:-$ROOT/vscode/runtime}"
INDEX_URL="${PIP_INDEX_URL:-https://pypi.org/simple}"
ONLY="${1:-}"

mkdir -p "$CACHE" "$DEST"

download() {
  local url="$1" out="$2"
  if [[ -s "$out" ]]; then
    return
  fi
  echo "下载 $url"
  curl -fL --retry 3 --retry-delay 2 -o "$out.partial" "$url"
  mv "$out.partial" "$out"
}

pip_install_conan() {
  local site="$1"
  shift
  local plat
  mkdir -p "$site"
  for plat in "$@"; do
    echo "安装 conan==${CONAN_VERSION}  (platform=${plat})"
    if python3 -m pip install \
      --isolated \
      --index-url "$INDEX_URL" \
      --trusted-host pypi.org \
      --trusted-host files.pythonhosted.org \
      --disable-pip-version-check \
      --no-compile \
      --python-version "$PYTHON_XY" \
      --implementation cp \
      --abi "cp${PYTHON_XY//./}" \
      --platform "$plat" \
      --only-binary=:all: \
      --upgrade \
      --target "$site" \
      "conan==${CONAN_VERSION}"; then
      return 0
    fi
    echo "platform=${plat} 失败，尝试下一组标签" >&2
  done
  echo "无法为 $site 安装 Conan ${CONAN_VERSION}" >&2
  return 1
}

prune_python() {
  local root="$1" family="$2"
  if [[ "$family" == "windows" ]]; then
    rm -rf \
      "$root/Lib/test" \
      "$root/Lib/idlelib" \
      "$root/Lib/tkinter" \
      "$root/Lib/turtledemo" \
      "$root/Lib/ensurepip" \
      "$root/Lib/venv" \
      "$root/include" \
      "$root/tcl" \
      "$root/libs" \
      "$root/share"
  else
    local py="$root/lib/python${PYTHON_XY}"
    rm -rf \
      "$py/test" \
      "$py/idlelib" \
      "$py/tkinter" \
      "$py/turtledemo" \
      "$py/ensurepip" \
      "$py/venv" \
      "$root/include" \
      "$root/share" \
      "$root/lib/pkgconfig"
    rm -rf "$root/lib"/config-"${PYTHON_XY}"-*
  fi
  find "$root" \( -type d -name '__pycache__' -o -name '*.pyc' -o -name '*.pyo' \) -prune -exec rm -rf {} + 2>/dev/null || true
}

write_manifest() {
  local dest="$1"
  cat > "$dest/manifest.json" <<EOF
{
  "python": "${PYTHON_VERSION}",
  "python_build": "${PYTHON_BUILD}",
  "conan": "${CONAN_VERSION}"
}
EOF
}

# vscode-platform | python-build-standalone triple | pip tags | family
PLATFORMS=(
  "linux-x64|x86_64-unknown-linux-gnu|manylinux2014_x86_64 manylinux_2_17_x86_64|unix"
  "linux-arm64|aarch64-unknown-linux-gnu|manylinux2014_aarch64 manylinux_2_17_aarch64|unix"
  "darwin-x64|x86_64-apple-darwin|macosx_11_0_x86_64 macosx_10_13_x86_64 macosx_10_9_x86_64|unix"
  "darwin-arm64|aarch64-apple-darwin|macosx_11_0_arm64 macosx_12_0_arm64 macosx_11_0_universal2|unix"
  "win32-x64|x86_64-pc-windows-msvc|win_amd64|windows"
)

for spec in "${PLATFORMS[@]}"; do
  IFS='|' read -r vscode_plat pbs_plat pip_tags family <<< "$spec"
  if [[ -n "$ONLY" && "$ONLY" != "$vscode_plat" && "$ONLY" != "all" ]]; then
    continue
  fi

  archive="cpython-${PYTHON_VERSION}+${PYTHON_BUILD}-${pbs_plat}-install_only_stripped.tar.gz"
  url="https://github.com/astral-sh/python-build-standalone/releases/download/${PYTHON_BUILD}/${archive}"
  tarpath="$CACHE/$archive"
  download "$url" "$tarpath"

  dest="$DEST/$vscode_plat"
  echo "解压到 $dest"
  rm -rf "$dest"
  mkdir -p "$dest"
  tar -xzf "$tarpath" -C "$dest"
  if [[ ! -d "$dest/python" ]]; then
    echo "压缩包未包含 python/ 目录: $archive" >&2
    exit 1
  fi

  if [[ "$family" == "windows" ]]; then
    site="$dest/python/Lib/site-packages"
  else
    site="$dest/python/lib/python${PYTHON_XY}/site-packages"
  fi
  # shellcheck disable=SC2086
  pip_install_conan "$site" $pip_tags
  if [[ ! -d "$site/conan" ]]; then
    echo "Conan 未安装到 $site" >&2
    exit 1
  fi

  prune_python "$dest/python" "$family"
  cat > "$site/sitecustomize.py" <<'PY'
import site
site.ENABLE_USER_SITE = False
PY
  write_manifest "$dest"

  if [[ "$family" != "windows" ]]; then
    chmod a+x "$dest/python/bin/python3" "$dest/python/bin/python${PYTHON_XY}" 2>/dev/null || true
  fi
  echo "完成 $vscode_plat"
done

host_python="$DEST/linux-x64/python/bin/python3"
if [[ -x "$host_python" && "$(uname -s)" == "Linux" && "$(uname -m)" == "x86_64" ]]; then
  echo "校验内置 Conan："
  "$host_python" -m conans.conan --version
fi
echo "runtime 已写入 $DEST"
