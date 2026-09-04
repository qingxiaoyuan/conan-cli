// 纯参数构造函数：被 extension.js 和 args.test.js 共用。
// 只依赖传入的 message 对象，不依赖 vscode 模块，便于用 node 直接测试。

function analyzeArgs(message) {
  const args = ['analyze'];
  if (message.os) args.push('--os', message.os);
  if (message.arch) args.push('--arch', message.arch);
  if (message.buildType) args.push('--build-type', String(message.buildType));
  return args;
}

function installArgs(message) {
  const args = ['install'];
  if (message.os) args.push('--os', message.os);
  if (message.arch) args.push('--arch', message.arch);
  if (message.buildType) args.push('--build-type', String(message.buildType));
  if (message.outputFolder) args.push('--output-folder', String(message.outputFolder));
  return args;
}

function configSetArgs(payload) {
  const args = ['config', 'set'];
  args.push('--name', payload.name || 'nexus');
  if (payload.url) args.push('--url', payload.url);
  if (payload.username) args.push('--username', payload.username);
  return args;
}

function pushDirFlags(args, flag, value) {
  const list = Array.isArray(value) ? value : String(value || '').split(/[,;\n]/);
  for (const item of list) {
    const trimmed = String(item || '').trim();
    if (trimmed) args.push(flag, trimmed);
  }
}

function settingsSetArgs(payload) {
  const args = ['settings', 'set'];
  const map = {
    name: '--name', qt: '--qt', compiler: '--compiler', compilerVersion: '--compiler-version',
    os: '--os', arch: '--arch', buildType: '--build-type',
    publishOs: '--publish-os', publishArch: '--publish-arch', publishBuildType: '--publish-build-type',
    channel: '--channel', outputFolder: '--output-folder',
    package: '--package', version: '--version',
  };
  for (const [key, flag] of Object.entries(map)) {
    if (payload[key]) args.push(flag, String(payload[key]));
  }
  pushDirFlags(args, '--lib-dir', payload.libDirs);
  pushDirFlags(args, '--include-dir', payload.includeDirs);
  if (payload.workspaces !== undefined) {
    const raw = Array.isArray(payload.workspaces) ? payload.workspaces.join(',') : payload.workspaces;
    if (String(raw || '').trim()) pushDirFlags(args, '--workspace', raw);
    else args.push('--workspace', '');
  }
  if (payload.noQt) args.push('--no-qt');
  return args;
}

function publishArgs(payload) {
  if (payload.all && payload.package) throw new Error('publish: --all 与 --package 互斥，不能同时指定');
  const args = ['publish'];
  if (payload.all) args.push('--all');
  else if (payload.package) args.push('--package', payload.package);
  if (payload.name) args.push('--name', payload.name);
  if (payload.version) args.push('--version', payload.version);
  if (payload.channel) args.push('--channel', payload.channel);
  if (payload.os) args.push('--os', payload.os);
  if (payload.arch) args.push('--arch', payload.arch);
  if (payload.buildType) args.push('--build-type', payload.buildType);
  if (payload.compiler) args.push('--compiler', payload.compiler);
  if (payload.compilerVersion) args.push('--compiler-version', payload.compilerVersion);
  if (payload.qt && !payload.noQt) args.push('--qt', payload.qt);
  if (payload.noQt) args.push('--no-qt');
  if (payload.note) args.push('--note', payload.note);
  if (payload.replace) args.push('--replace');
  pushDirFlags(args, '--lib-dir', payload.libDirs);
  pushDirFlags(args, '--include-dir', payload.includeDirs);
  return args;
}

module.exports = { analyzeArgs, installArgs, configSetArgs, settingsSetArgs, publishArgs };
