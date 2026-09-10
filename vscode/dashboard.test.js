const assert = require('assert');
const fs = require('fs');
const path = require('path');
const files = require('./dashboard/files');

const dir = path.join(__dirname, 'dashboard');
assert.ok(Array.isArray(files) && files.length, 'dashboard/files.js 不能为空');

const bundled = files
  .map((name) => {
    const full = path.join(dir, name);
    assert.ok(fs.existsSync(full), '缺少 ' + name);
    const src = fs.readFileSync(full, 'utf8');
    assert.ok(src.trim(), name + ' 不能为空');
    const firstCode = src.split('\n').find((line) => line.trim() && !line.trim().startsWith('//'));
    assert.ok(firstCode, name + ' 没有代码');
    assert.match(
      firstCode.trim(),
      /^(const|let|function|async function)\b/,
      name + ' 不是完整语句开头: ' + firstCode.trim()
    );
    return src;
  })
  .join('\n');

assert.doesNotThrow(() => new Function(bundled), '拼接后的 dashboard 脚本无法解析');
['renderCatalog', 'renderPublish', 'renderDoctor', 'renderDownload', 'renderSettings', 'pkg-block'].forEach((token) => {
  assert.ok(bundled.includes(token), '缺少 ' + token);
});
console.log('dashboard.test.js: all passed');
