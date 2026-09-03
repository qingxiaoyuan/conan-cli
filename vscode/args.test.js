// 运行方式：node vscode/args.test.js（零依赖，只用 node:assert）
const assert = require('assert');
const { analyzeArgs, installArgs, configSetArgs, settingsSetArgs, publishArgs } = require('./args');

assert.deepStrictEqual(analyzeArgs({}), ['analyze']);
assert.deepStrictEqual(analyzeArgs({ os: 'kylin', arch: 'x64', buildType: 'Release' }), [
  'analyze', '--os', 'kylin', '--arch', 'x64', '--build-type', 'Release',
]);

assert.deepStrictEqual(installArgs({}), ['install']);
assert.deepStrictEqual(installArgs({ os: 'linux', arch: 'arm64', buildType: 'Debug', outputFolder: 'out' }), [
  'install', '--os', 'linux', '--arch', 'arm64', '--build-type', 'Debug', '--output-folder', 'out',
]);

assert.deepStrictEqual(configSetArgs({}), ['config', 'set', '--name', 'nexus']);
assert.deepStrictEqual(configSetArgs({ name: 'prod', url: 'https://nexus.test/repo', username: 'alice' }), [
  'config', 'set', '--name', 'prod', '--url', 'https://nexus.test/repo', '--username', 'alice',
]);

assert.deepStrictEqual(settingsSetArgs({}), ['settings', 'set']);
assert.deepStrictEqual(settingsSetArgs({
  name: 'qtutils', qt: '6.8', compiler: 'gcc', compilerVersion: '13',
  os: 'kylin', arch: 'x64', buildType: 'Release',
  publishOs: 'linux', publishArch: 'arm64', publishBuildType: 'Debug',
  channel: 'dev', outputFolder: 'conan',
}), [
  'settings', 'set',
  '--name', 'qtutils', '--qt', '6.8', '--compiler', 'gcc', '--compiler-version', '13',
  '--os', 'kylin', '--arch', 'x64', '--build-type', 'Release',
  '--publish-os', 'linux', '--publish-arch', 'arm64', '--publish-build-type', 'Debug',
  '--channel', 'dev', '--output-folder', 'conan',
]);
// 空值字段不应产生 flag
assert.deepStrictEqual(settingsSetArgs({ name: '', qt: undefined }), ['settings', 'set']);

assert.deepStrictEqual(publishArgs({}), ['publish']);
assert.deepStrictEqual(publishArgs({
  name: 'qtutils', version: '1.0', channel: 'dev', os: 'kylin', arch: 'x64',
  buildType: 'Release', compiler: 'gcc', compilerVersion: '13', qt: '6.8', note: 'hello',
}), [
  'publish', '--name', 'qtutils', '--version', '1.0', '--channel', 'dev',
  '--os', 'kylin', '--arch', 'x64', '--build-type', 'Release',
  '--compiler', 'gcc', '--compiler-version', '13', '--qt', '6.8', '--note', 'hello',
]);

console.log('args.test.js: all passed');
