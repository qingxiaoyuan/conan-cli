// 诊断页：Conan / 配置 / 仓库 / 平台检查。

const CHECKS = {
  conan: { title: 'Conan 是否可用', ok: '本机可以运行 Conan 2。', fail: '找不到 Conan，请先安装 Conan 2。' },
  project_config: { title: '项目配置', ok: '已有项目配置。', fail: '选平台并拉取依赖时会自动创建。' },
  conanfile: { title: '配方文件', ok: '已有 conanfile。', fail: '缺少配方。拉取依赖时会生成最简 conanfile.txt。' },
  global_remote: { title: '仓库登录', ok: 'Nexus 地址和账号已保存。', fail: '请到设置里填写地址、用户名并保存登录。' },
  platform: { title: '目标平台', ok: '已选择制品要跑的系统和架构。', fail: '到「拉取依赖」页点选操作系统和架构即可。' },
};

function checkMeta(name) {
  return CHECKS[name] || { title: name, ok: '检查通过。', fail: '检查未通过。' };
}

function friendlyDetail(name, detail) {
  const text = String(detail || '').trim();
  if (name === 'conan' && /Conan version/i.test(text)) return text.replace(/^Conan version/i, '版本');
  if (name === 'project_config' && text.includes('project.yaml')) return '路径：' + text;
  return text;
}

function renderDoctor() {
  const checks = ((state.doctor && state.doctor.checks) || []).filter((c) => !SKIP_CHECKS[c.name]);
  const failed = checks.filter((c) => !c.ok).length;
  $('doctor-summary').innerHTML = checks.length
    ? failed
      ? `<strong class="warn">${failed} 项需要处理</strong><span class="hint">共 ${checks.length} 项</span>`
      : `<strong class="ok">就绪</strong><span class="hint">共 ${checks.length} 项</span>`
    : '';
  $('doctor-list').innerHTML =
    checks
      .map((c) => {
        const meta = checkMeta(c.name);
        return `<div class="check"><div class="check-icon ${c.ok ? 'ok' : 'fail'}">${c.ok ? '✓' : '!'}</div><div><div class="check-title">${esc(meta.title)}</div><div class="check-desc">${esc(c.ok ? meta.ok : meta.fail)}</div></div><div class="check-result">${esc(friendlyDetail(c.name, c.detail))}</div></div>`;
      })
      .join('') || '<div class="hint" style="padding:16px">还没有诊断结果。</div>';
  $('raw-out').textContent = state.raw || '';
}
