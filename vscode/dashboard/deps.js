// 依赖表与缺制品卡片。拉取页和依赖页共用同一份 analyze 结果。

function isMissingStatus(s) {
  return s === 'missing_binary' || s === 'missing_package' || s === 'missing_version';
}

function statusLabel(v) {
  return (
    {
      found: '已找到',
      missing_binary: '无匹配二进制',
      missing_package: '无此包',
      missing_version: '无此版本',
      mismatch: '不一致',
      unknown: '未查询',
    }[v] || v
  );
}

function analyzeRows() {
  return (state.analyze && state.analyze.dependencies) || [];
}

function depRow(r) {
  const detail = isMissingStatus(r.status)
    ? (r.detail || '') + (r.detail ? ' · ' : '') + '见下方缺制品卡片'
    : r.detail || '';
  return `<tr class="${r.status === 'found' || r.status === 'unknown' ? '' : 'warn-row'}"><td class="mono">${esc(r.reference)}</td><td class="${r.status === 'found' ? 'ok' : 'warn'}">${esc(statusLabel(r.status))}</td><td class="detail">${esc(detail)}</td></tr>`;
}

function renderDepTable(el) {
  if (!el) return;
  const empty = '<tr><td colspan="3" class="detail">选好平台后点「检查依赖」或「拉取依赖」</td></tr>';
  el.innerHTML = analyzeRows().map(depRow).join('') || empty;
}

function paintMissCard(el, rows) {
  if (!el) return;
  const miss = (rows || []).filter((r) => isMissingStatus(r.status));
  if (!miss.length) {
    el.hidden = true;
    el.innerHTML = '';
    return;
  }
  const refs = miss.map((r) => r.reference).filter(Boolean);
  const human = formatComboHuman(
    osSel,
    archSel,
    $('p-compiler').value,
    $('p-compiler-ver').value,
    $('p-qt').value,
    false,
    btSel
  );
  el.hidden = false;
  el.innerHTML =
    `<h3>仓库没有这套制品</h3>` +
    `<p>当前组合：<strong>${esc(human)}</strong></p>` +
    `<p>缺失 <span class="mono">${esc(refs.join('、'))}</span>。请联系你们的制品负责人；不要在本机编译。</p>` +
    `<div class="actions" style="margin-top:12px"><button type="button" class="btn primary" data-fix-combo>改组合</button></div>` +
    `<p class="hint">不会执行 --build=missing。原始 Conan 输出可在诊断页展开。</p>`;
  el.querySelector('[data-fix-combo]').onclick = () => {
    comboEditOpen = true;
    show('download');
    render();
    const bar = $('consume-combo-edit');
    if (bar) bar.scrollIntoView({ behavior: 'smooth', block: 'start' });
  };
}

function renderDeps() {
  renderDepTable($('dep-rows-full'));
  paintMissCard($('miss-card-deps'), analyzeRows());
}
