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
assert.deepStrictEqual(settingsSetArgs({ libDirs: 'build/Release, src/lib', includeDirs: ['include/foo'] }), [
  'settings', 'set', '--lib-dir', 'build/Release', '--lib-dir', 'src/lib', '--include-dir', 'include/foo',
]);
// 空值字段不应产生 flag
assert.deepStrictEqual(settingsSetArgs({ name: '', qt: undefined }), ['settings', 'set']);
// workspaces：非空展开为重复 flag，空串恢复默认，未传不产生 flag
assert.deepStrictEqual(settingsSetArgs({ workspaces: 'packages/*, src/libs/*' }), [
  'settings', 'set', '--workspace', 'packages/*', '--workspace', 'src/libs/*',
]);
assert.deepStrictEqual(settingsSetArgs({ workspaces: '' }), ['settings', 'set', '--workspace', '']);

assert.deepStrictEqual(publishArgs({}), ['publish']);
assert.deepStrictEqual(publishArgs({
  name: 'qtutils', version: '1.0', channel: 'dev', os: 'kylin', arch: 'x64',
  buildType: 'Release', compiler: 'gcc', compilerVersion: '13', qt: '6.8', note: 'hello',
}), [
  'publish', '--name', 'qtutils', '--version', '1.0', '--channel', 'dev',
  '--os', 'kylin', '--arch', 'x64', '--build-type', 'Release',
  '--compiler', 'gcc', '--compiler-version', '13', '--qt', '6.8', '--note', 'hello',
]);
assert.deepStrictEqual(publishArgs({ libDirs: ['build/Release'] }), ['publish', '--lib-dir', 'build/Release']);
assert.deepStrictEqual(publishArgs({ package: 'beta', name: 'beta', version: '1.0' }), [
  'publish', '--package', 'beta', '--name', 'beta', '--version', '1.0',
]);
assert.deepStrictEqual(publishArgs({ package: 'plainlib', noQt: true, qt: '6.8' }), [
  'publish', '--package', 'plainlib', '--no-qt',
]);
assert.deepStrictEqual(publishArgs({ all: true, os: 'linux', arch: 'x64', channel: 'dev' }), [
  'publish', '--all', '--channel', 'dev', '--os', 'linux', '--arch', 'x64',
]);
// 替换旧版本
assert.deepStrictEqual(publishArgs({ package: 'beta', version: '2.0', replace: true }), [
  'publish', '--package', 'beta', '--version', '2.0', '--replace',
]);
// --all 与 --package 互斥，同给时抛错（webview 已保证互斥，这里做防御）
assert.throws(() => publishArgs({ all: true, package: 'beta' }), /互斥/);

console.log('args.test.js: all passed');
