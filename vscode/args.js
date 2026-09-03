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

function settingsSetArgs(payload) {
  const args = ['settings', 'set'];
  const map = {
    name: '--name', qt: '--qt', compiler: '--compiler', compilerVersion: '--compiler-version',
    os: '--os', arch: '--arch', buildType: '--build-type',
    publishOs: '--publish-os', publishArch: '--publish-arch', publishBuildType: '--publish-build-type',
    channel: '--channel', outputFolder: '--output-folder',
  };
  for (const [key, flag] of Object.entries(map)) {
    if (payload[key]) args.push(flag, String(payload[key]));
  }
  return args;
}

function publishArgs(payload) {
  const args = ['publish'];
  if (payload.name) args.push('--name', payload.name);
  if (payload.version) args.push('--version', payload.version);
  if (payload.channel) args.push('--channel', payload.channel);
  if (payload.os) args.push('--os', payload.os);
  if (payload.arch) args.push('--arch', payload.arch);
  if (payload.buildType) args.push('--build-type', payload.buildType);
  if (payload.compiler) args.push('--compiler', payload.compiler);
  if (payload.compilerVersion) args.push('--compiler-version', payload.compilerVersion);
  if (payload.qt) args.push('--qt', payload.qt);
  if (payload.note) args.push('--note', payload.note);
  return args;
}

module.exports = { analyzeArgs, installArgs, configSetArgs, settingsSetArgs, publishArgs };
