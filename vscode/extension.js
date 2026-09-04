const vscode = require('vscode');
const cp = require('child_process');
const crypto = require('crypto');
const fs = require('fs');
const path = require('path');
const cliArgs = require('./args');

const output = vscode.window.createOutputChannel('Conan CLI');
let dashboardPanel;
let extensionPath = '';
let connectionProbe;
let connectionProbePromise;
const refreshJobs = new WeakMap();
const CONNECTION_TTL = 30 * 1000;

// install / publish 会下载或上传大体积制品，动辄超过两分钟，用独立的
// 长超时；其余命令保持 120 秒。两个阈值都可以在设置里调整。
const LONG_RUNNING_ACTIONS = new Set(['install', 'publish']);

function timeoutMsFor(args) {
  const config = vscode.workspace.getConfiguration('conanCli');
  const action = String(args[0] || '');
  if (LONG_RUNNING_ACTIONS.has(action)) {
    const minutes = Number(config.get('longCommandTimeoutMinutes', 30));
    return Math.max(1, Number.isFinite(minutes) ? minutes : 30) * 60 * 1000;
  }
  const seconds = Number(config.get('commandTimeoutSeconds', 120));
  return Math.max(1, Number.isFinite(seconds) ? seconds : 120) * 1000;
}

class SidebarProvider {
  constructor(extensionUri) {
    this.extensionUri = extensionUri;
    this.view = undefined;
  }
  resolveWebviewView(view) {
    this.view = view;
    view.webview.options = {
      enableScripts: true,
      localResourceRoots: [this.extensionUri],
    };
    view.webview.html = loadWebview('sidebar.html', view.webview);
    view.webview.onDidReceiveMessage((message) => {
      const root = requireWorkspace();
      if (root) handleWebviewMessage({ webview: view.webview }, root, message);
    });
    const root = workspaceRoot();
    if (root) {
      refreshState({ webview: view.webview }, root).then(() => probeConnection(root, { webview: view.webview }));
    }
  }
  refresh(root) {
    if (this.view && root) refreshState({ webview: this.view.webview }, root);
  }
}

function workspaceRoot() { return vscode.workspace.workspaceFolders?.[0]?.uri.fsPath; }

function requireWorkspace() {
  const root = workspaceRoot();
  if (!root) vscode.window.showErrorMessage('请先打开一个 Conan 项目文件夹。');
  return root;
}

function bundledBinary() {
  const name = process.platform === 'win32' ? 'conan-cli.exe' : 'conan-cli';
  return path.join(extensionPath, 'bin', `${process.platform}-${process.arch}`, name);
}

function bundledPython() {
  const root = path.join(extensionPath, 'runtime', `${process.platform}-${process.arch}`, 'python');
  return process.platform === 'win32'
    ? path.join(root, 'python.exe')
    : path.join(root, 'bin', 'python3');
}

function executablePath(root) {
  const configured = String(vscode.workspace.getConfiguration('conanCli').get('binary', '') || '').trim();
  const useBundled = !configured || configured === 'conan-cli';
  if (!useBundled) {
    const expanded = configured.replace(/\$\{workspaceFolder\}/g, root || '');
    if (path.isAbsolute(expanded) || expanded.includes('/') || expanded.includes('\\')) {
      return path.resolve(root || '', expanded);
    }
    return expanded;
  }
  const bundled = bundledBinary();
  if (bundled && fs.existsSync(bundled)) return bundled;
  return 'conan-cli';
}

function cliEnvironment(root, extra = {}) {
  const env = { ...process.env, ...extra };
  const configured = String(vscode.workspace.getConfiguration('conanCli').get('conanBinary', '') || '').trim();
  if (configured) {
    env.CONAN_BIN = configured.replace(/\$\{workspaceFolder\}/g, root || '');
    return env;
  }
  const python = bundledPython();
  if (python && fs.existsSync(python) && !env.CONAN_BIN && !env.CONAN_CLI_BUNDLED_PYTHON) {
    env.CONAN_CLI_BUNDLED_PYTHON = python;
    env.PATH = path.dirname(python) + path.delimiter + (env.PATH || '');
    env.PYTHONNOUSERSITE = '1';
  }
  return env;
}

function runCli(root, args, environment = {}) {
  return new Promise((resolve) => {
    const command = executablePath(root);
    const fullArgs = ['--json', ...args];
    output.appendLine(`$ ${command} ${fullArgs.join(' ')}`);
    const child = cp.spawn(command, fullArgs, { cwd: root, env: cliEnvironment(root, environment), windowsHide: true });
    let stdout = '';
    let stderr = '';
    let settled = false;
    const timeoutMs = timeoutMsFor(args);
    const finish = (response) => {
      if (settled) return;
      settled = true;
      clearTimeout(timer);
      resolve(response);
    };
    const timer = setTimeout(() => {
      child.kill();
      finish({
        ok: false,
        action: args[0] || 'command',
        error: `conan-cli 执行超时（${Math.round(timeoutMs / 1000)} 秒）。可在设置 conanCli.commandTimeoutSeconds / conanCli.longCommandTimeoutMinutes 中调整。`,
        exit_code: 124,
      });
    }, timeoutMs);
    child.stdout.on('data', (chunk) => { stdout += chunk.toString(); });
    child.stderr.on('data', (chunk) => { stderr += chunk.toString(); });
    child.on('error', (error) => {
      const missing = error && error.code === 'ENOENT';
      finish({
        ok: false,
        action: args[0] || 'command',
        error: missing
          ? `找不到 conan-cli（${command}）。请安装带 CLI 的插件包，或在设置 conanCli.binary 填写绝对路径。`
          : (error && error.message) || String(error),
        exit_code: 1,
      });
    });
    child.on('close', (code) => {
      if (settled) return;
      if (stderr.trim()) output.appendLine(stderr.trimEnd());
      let response;
      try { response = JSON.parse(stdout); }
      catch (_) { response = { ok: false, action: args[0] || 'command', error: stdout.trim() || stderr.trim() || `conan-cli 退出码 ${code}`, exit_code: code || 1 }; }
      if (response.message && !response.error && !response.ok) response.error = response.message;
      response.exit_code = response.exit_code ?? code ?? 0;
      finish(response);
    });
  });
}

function displayResponse(response, successMessage) {
  output.appendLine(JSON.stringify(response, null, 2));
  if (vscode.workspace.getConfiguration('conanCli').get('showCommandOutput', true)) output.show(true);
  if (response.ok) vscode.window.showInformationMessage(successMessage || `${response.action} 已完成。`);
  else vscode.window.showErrorMessage(response.error || response.message || `${response.action} 失败。`, '查看输出').then((choice) => { if (choice === '查看输出') output.show(true); });
}

async function execute(root, args, successMessage, environment) {
  const response = await runCli(root, args, environment);
  displayResponse(response, successMessage);
  return response;
}

async function addDependency(root) {
  const dependency = await vscode.window.showInputBox({ prompt: 'Conan 包引用', placeHolder: 'fmt/10.2.1', validateInput: (value) => value.trim() ? undefined : '请输入包引用。' });
  if (dependency) await execute(root, ['add', dependency.trim()], `已添加 ${dependency.trim()}。`);
}

async function probeConnection(root, panel) {
  const key = root;
  if (connectionProbe && connectionProbe.key === key && Date.now() - connectionProbe.checkedAt < CONNECTION_TTL) {
    if (panel && connectionProbe) panel.webview.postMessage({ type: 'probe', probe: connectionProbe });
    return connectionProbe;
  }
  if (connectionProbePromise) {
    const result = await connectionProbePromise;
    if (panel) panel.webview.postMessage({ type: 'probe', probe: result });
    return result;
  }
  if (panel) panel.webview.postMessage({ type: 'busy', label: '正在检查仓库连接…' });
  connectionProbePromise = runCli(root, ['config', 'test']).then((response) => {
    connectionProbe = {
      key,
      checkedAt: Date.now(),
      ok: !!response.ok,
      message: response.ok ? (response.message || '仓库可达') : (response.error || response.message || '连接失败'),
    };
    return connectionProbe;
  }).finally(() => { connectionProbePromise = undefined; });
  try {
    const result = await connectionProbePromise;
    if (panel) panel.webview.postMessage({ type: 'probe', probe: result });
    return result;
  } finally {
    if (panel) panel.webview.postMessage({ type: 'busy', label: '' });
  }
}

async function refreshState(panel, root, extra = {}) {
  if (!panel) return;
  const existing = refreshJobs.get(panel);
  if (existing) return existing;
  const job = (async () => {
  panel.webview.postMessage({ type: 'busy', label: '正在刷新…' });
  const [status, doctor, analyze] = await Promise.all([
    runCli(root, ['status']),
    runCli(root, ['doctor']),
    runCli(root, ['analyze']),
  ]);
  const raw = [status.output, doctor.output, analyze.output].filter(Boolean).join('\n');
  panel.webview.postMessage({
    type: 'state',
    status: status.data || {},
    doctor,
    analyze: analyze.data || {},
    analyzeResponse: analyze,
    raw,
    ...(connectionProbe ? { probe: connectionProbe } : {}),
    ...extra,
  });
  })();
  refreshJobs.set(panel, job);
  try {
    return await job;
  } finally {
    if (refreshJobs.get(panel) === job) refreshJobs.delete(panel);
  }
}

async function handleWebviewMessage(panel, root, message) {
  if (!panel || !message) return;
  if (message.type === 'open') {
    openDashboard(root, message.view);
    return;
  }
  const busy = (label) => panel.webview.postMessage({ type: 'busy', label });
  switch (message.type) {
    case 'refresh':
      await refreshState(panel, root);
      return;
    case 'init':
      busy('正在初始化…');
      await execute(root, ['init'], '项目已初始化。');
      break;
    case 'scan':
      busy('正在扫描…');
      await execute(root, ['scan'], '扫描完成。');
      break;
    case 'scan-fill': {
      busy('正在尝试获取本机编译器 / Qt…');
      const scanned = await runCli(root, ['scan']);
      panel.webview.postMessage({ type: 'scan-fill', scan: (scanned.data && scanned.data.scan) || {} });
      panel.webview.postMessage({ type: 'busy', label: '' });
      const finding = scanned.data && scanned.data.scan;
      const hasQt = finding && finding.qt_installs && finding.qt_installs.length;
      const hasCompiler = finding && finding.compiler && finding.compiler.id;
      if (hasQt || hasCompiler) vscode.window.showInformationMessage('已填入探测到的编译器 / Qt，请核对后再用。');
      else vscode.window.showWarningMessage('本机没有探测到可用的编译器或 Qt，请手填。');
      return;
    }
    case 'recipe-generate': {
      busy('正在生成 conanfile.py…');
      const args = ['recipe', 'generate', '--kind', message.kind || 'consume'];
      if (message.name) args.push('--name', String(message.name));
      if (message.version) args.push('--version', String(message.version));
      if (message.qt) args.push('--qt', String(message.qt));
      if (message.force) args.push('--force');
      let response = await runCli(root, args);
      if (!response.ok && /覆盖/.test(String(response.error || ''))) {
        const choice = await vscode.window.showWarningMessage(response.error || '已有配方', '覆盖');
        if (choice === '覆盖') {
          args.push('--force');
          response = await runCli(root, args);
        } else {
          panel.webview.postMessage({ type: 'busy', label: '' });
          return;
        }
      }
      displayResponse(response, response.message || '已生成 conanfile.py。');
      break;
    }
    case 'scan-apply':
      busy('正在写入扫描结果…');
      await execute(root, ['scan', '--apply'], '已采纳扫描结果。');
      break;
    case 'analyze':
      busy('正在分析依赖…');
      await execute(root, cliArgs.analyzeArgs(message), '分析完成。');
      break;
    case 'install':
      busy('正在拉取 Conan 依赖…');
      await execute(root, cliArgs.installArgs(message), 'Conan 依赖已拉取。');
      break;
    case 'catalog': {
      busy('正在查询仓库…');
      const args = ['catalog'];
      if (message.query) args.push(String(message.query));
      const response = await runCli(root, args);
      panel.webview.postMessage({
        type: 'catalog',
        catalog: response.data || {},
        error: response.ok ? '' : (response.error || response.message || '查询失败'),
      });
      panel.webview.postMessage({ type: 'busy', label: '' });
      return;
    }
    case 'add-ref':
      if (message.ref) await execute(root, ['add', String(message.ref)], `已添加 ${message.ref}。`);
      break;
    case 'doctor':
      busy('正在诊断…');
      await execute(root, ['doctor'], '诊断完成。');
      break;
    case 'add':
      await addDependency(root);
      break;
    case 'save-global': {
      const payload = message.payload || {};
      busy('正在保存全局设置…');
      const env = {};
      if (payload.password) env.CONAN_PASSWORD = payload.password;
      await execute(root, cliArgs.configSetArgs(payload), '全局设置已保存。', env);
      if (payload.password || env.CONAN_PASSWORD) await execute(root, ['config', 'login'], '已登录远程仓库。', env);
      connectionProbe = undefined;
      await probeConnection(root, panel);
      break;
    }
    case 'test':
      busy('正在测试连接…');
      connectionProbe = undefined;
      const probe = await probeConnection(root, panel);
      if (probe && probe.ok) vscode.window.showInformationMessage(probe.message || '仓库可达。');
      else vscode.window.showErrorMessage((probe && probe.message) || '仓库连接失败。');
      break;
    case 'save-project':
      busy('正在保存项目设置…');
      await execute(root, cliArgs.settingsSetArgs(message.payload || {}), '项目设置已保存。');
      break;
    case 'save-project-quiet': {
      const args = cliArgs.settingsSetArgs(message.payload || {});
      if (args.length > 2) await runCli(root, args);
      return;
    }
    case 'publish': {
      const isAll = !!(message.payload && message.payload.all);
      busy(isAll ? '正在发布全部组件…' : '正在更新配方并发布…');
      const response = await execute(root, cliArgs.publishArgs(message.payload || {}), isAll ? '组件发布完成。' : '已更新 conanfile.py 并发布。');
      panel.webview.postMessage({ type: 'publish-result', response });
      break;
    }
    default:
      break;
  }
  await refreshState(panel, root);
}

function loadWebview(file, webview) {
  const fs = require('fs');
  const nonce = crypto.randomBytes(16).toString('hex');
  return fs.readFileSync(path.join(__dirname, file), 'utf8').replaceAll('{{CSP_SOURCE}}', webview.cspSource).replaceAll('{{NONCE}}', nonce);
}

function openDashboard(root, view) {
  root = workspaceRoot();
  if (!root) return;
  if (dashboardPanel) {
    dashboardPanel.reveal(vscode.ViewColumn.One);
    refreshState(dashboardPanel, root).then(async () => {
      await probeConnection(root, dashboardPanel);
      if (view) dashboardPanel.webview.postMessage({ type: 'open-view', view });
    });
    return;
  }
  dashboardPanel = vscode.window.createWebviewPanel('conanCli.dashboard', 'Conan 控制台', vscode.ViewColumn.One, { enableScripts: true, retainContextWhenHidden: true });
  dashboardPanel.webview.html = loadWebview('dashboard.html', dashboardPanel.webview);
  dashboardPanel.webview.onDidReceiveMessage((message) => {
    const currentRoot = requireWorkspace();
    if (currentRoot) handleWebviewMessage(dashboardPanel, currentRoot, message);
  });
  dashboardPanel.onDidDispose(() => { dashboardPanel = undefined; });
  refreshState(dashboardPanel, root).then(async () => {
    await probeConnection(root, dashboardPanel);
    if (view) dashboardPanel.webview.postMessage({ type: 'open-view', view });
  });
}

function activate(context) {
  extensionPath = context.extensionPath;
  try {
    const bundled = bundledBinary();
    if (bundled && fs.existsSync(bundled) && process.platform !== 'win32') fs.chmodSync(bundled, 0o755);
    const python = bundledPython();
    if (python && fs.existsSync(python) && process.platform !== 'win32') fs.chmodSync(python, 0o755);
  } catch (_) { /* ignore */ }
  const sidebar = new SidebarProvider(context.extensionUri);
  context.subscriptions.push(vscode.workspace.onDidChangeWorkspaceFolders(() => {
    connectionProbe = undefined;
    connectionProbePromise = undefined;
    const root = workspaceRoot();
    if (root) {
      sidebar.refresh(root);
      if (dashboardPanel) refreshState(dashboardPanel, root);
    }
  }));
  context.subscriptions.push(vscode.window.registerWebviewViewProvider('conanCli.actions', sidebar, {
    webviewOptions: { retainContextWhenHidden: true },
  }));
  context.subscriptions.push(output);
  const status = vscode.window.createStatusBarItem(vscode.StatusBarAlignment.Left, 100);
  status.text = '$(package) Conan';
  status.tooltip = '打开 Conan 控制台';
  status.command = 'conanCli.openDashboard';
  status.show();
  context.subscriptions.push(status);
  const register = (command, handler) => context.subscriptions.push(vscode.commands.registerCommand(command, async () => {
    const root = requireWorkspace();
    if (root) await handler(root);
    sidebar.refresh(root);
    if (root && dashboardPanel && command !== 'conanCli.openDashboard') await refreshState(dashboardPanel, root);
  }));
  register('conanCli.openDashboard', openDashboard);
  register('conanCli.init', (root) => execute(root, ['init'], 'Conan 项目已初始化。'));
  register('conanCli.install', (root) => execute(root, ['install'], '依赖下载完成。'));
  register('conanCli.scan', (root) => execute(root, ['scan'], '扫描完成。'));
  register('conanCli.analyze', (root) => execute(root, ['analyze'], '依赖分析完成。'));
  register('conanCli.doctor', async (root) => {
    const response = await execute(root, ['doctor'], '诊断完成。');
    if (response.checks) {
      const failed = response.checks.filter((check) => !check.ok).length;
      status.text = failed ? `$(warning) Conan：${failed} 个问题` : '$(pass) Conan：正常';
    }
  });
  register('conanCli.add', addDependency);
  register('conanCli.publish', (root) => execute(root, ['publish', '--dry-run'], '已生成发布预览，请在控制台确认。'));
  context.subscriptions.push(vscode.commands.registerCommand('conanCli.showOutput', () => output.show(true)));
  context.subscriptions.push(vscode.commands.registerCommand('conanCli.refresh', () => sidebar.refresh(workspaceRoot())));
}

function deactivate() { output.dispose(); }
module.exports = { activate, deactivate };
