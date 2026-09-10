// 控制台共享状态与工具。dashboard/*.js 由 extension.js 按序拼进同一脚本作用域。
const api = acquireVsCodeApi();

const OS = [
  ['windows', 'Windows'],
  ['linux', 'Linux'],
  ['kylin', '麒麟'],
];

// 展示用 Conan arch 字典；提交值映射到现插件 x86/x64/arm/arm64（其余原样，须为合法 Conan arch）
const ARCH_DICT = [
  { id: 'x86', label: 'x86', aliases: ['x86', 'i386', 'i686'] },
  { id: 'x64', label: 'x86_64', aliases: ['x86_64', 'x64', 'amd64', 'x86-64'] },
  { id: 'arm', label: 'armv7', aliases: ['armv7', 'arm', 'arm32', 'armhf', 'armel'] },
  { id: 'arm64', label: 'aarch64（armv8）', aliases: ['aarch64', 'armv8', 'arm64'] },
  { id: 'ppc32', label: 'ppc32', aliases: ['ppc32', 'ppc'] },
  { id: 'ppc64', label: 'ppc64', aliases: ['ppc64'] },
  { id: 'ppc64le', label: 'ppc64le', aliases: ['ppc64le'] },
  { id: 's390x', label: 's390x', aliases: ['s390x'] },
  { id: 'mips', label: 'mips', aliases: ['mips'] },
  { id: 'mips64', label: 'mips64', aliases: ['mips64'] },
  { id: 'sparc', label: 'sparc', aliases: ['sparc'] },
  { id: 'sparcv9', label: 'sparcv9', aliases: ['sparcv9'] },
  { id: 'riscv64', label: 'riscv64', aliases: ['riscv64'] },
];
const ARCH_PLUGIN = new Set(['x86', 'x64', 'arm', 'arm64']);
const ARCH_BY_ALIAS = (() => {
  const m = {};
  ARCH_DICT.forEach((e) => {
    e.aliases.forEach((a) => {
      m[a.toLowerCase()] = e.id;
    });
    m[e.id] = e.id;
  });
  m['armv8'] = 'arm64';
  return m;
})();
const BUILD = [
  ['Debug', 'Debug'],
  ['Release', 'Release'],
];
const SKIP_CHECKS = { profiles: 1, remotes: 1, configured_remote: 1, manifest_dependencies: 1 };

let comboEditOpen = false;
let pubComboEditOpen = false;
let archCustomDraft = '';
let pubArchCustomDraft = '';
let archCustomErr = '';
let pubArchCustomErr = '';
let state = { status: {}, doctor: {}, analyze: {}, catalog: {} };
let osSel = '';
let archSel = '';
let btSel = '';
let pubOs = '';
let pubArch = '';
let pubBt = '';
let openPkg = '';
let pubSelected = '';
let publishing = false;
let loopActive = false;
let modalAction = null;
const pubChecked = new Set();
const pubProgress = {};
const pendingResolve = {};

function $(id) {
  return document.getElementById(id);
}

function project() {
  return (state.status && state.status.project) || {};
}

function primaryPackage() {
  return (project().packages || [])[0] || {};
}

function joinDirs(values) {
  return (values || []).join(', ');
}

function global() {
  return (state.status && state.status.global) || { nexus: {} };
}

function outDir() {
  return ($('p-out').value.trim() || 'conan').replace(/\\/g, '/');
}

// 组件清单数据源：优先 status.packages（含 workspace 自动发现与产物探测）；
// 为空时退回 project.packages[] / 项目名，保持单组件项目的现有体验。
function packagesList() {
  const fromStatus = (state.status && state.status.packages) || [];
  if (fromStatus.length) return fromStatus;
  const p = project();
  const fallback = (name, spec) => ({
    name,
    version: (spec && spec.version) || '',
    source: 'declared',
    lib_dirs: (spec && spec.lib_dirs) || [],
    include_dirs: (spec && spec.include_dirs) || [],
    no_qt: !!(spec && spec.no_qt),
    has_artifacts: false,
    has_recipe: false,
  });
  const specs = p.packages || [];
  if (specs.length) return specs.map((s) => fallback(s.name, s));
  if (p.name) return [fallback(p.name, null)];
  return ((state.status && state.status.package_candidates) || []).map((x) => fallback(x.name, null));
}

function esc(v) {
  return String(v ?? '').replace(
    /[&<>"']/g,
    (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#039;' })[c]
  );
}
