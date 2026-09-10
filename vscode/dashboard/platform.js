// 操作系统 / 架构 / 目标组合。拉取页与发布页共用。

function displayOs(v) {
  return { windows: 'Windows', linux: 'Linux', kylin: '麒麟' }[v] || v || '-';
}

function normalizeArch(v) {
  const key = String(v || '')
    .trim()
    .toLowerCase();
  if (!key) return '';
  if (ARCH_BY_ALIAS[key]) return ARCH_BY_ALIAS[key];
  return key;
}

function archEntry(v) {
  const id = normalizeArch(v);
  return ARCH_DICT.find((e) => e.id === id) || null;
}

function displayArch(v) {
  const e = archEntry(v);
  if (e) return e.label;
  const raw = String(v || '').trim();
  return raw || '-';
}

function isLegalConanArch(v) {
  const raw = String(v || '').trim();
  if (!raw) return false;
  const n = normalizeArch(raw);
  if (ARCH_PLUGIN.has(n)) return true;
  if (ARCH_DICT.some((e) => e.id === n)) return true;
  return !!ARCH_BY_ALIAS[raw.toLowerCase()];
}

function isArchSubmittable(v) {
  return isLegalConanArch(v);
}

function compilerBit(compiler, ver) {
  const c = (compiler || '').trim();
  const v = (ver || '').trim();
  if (!c && !v) return '编译器未填';
  return (c || 'compiler') + (v ? ' ' + v : '');
}

function qtBit(qt, noQt) {
  if (noQt) return '无 Qt';
  const q = (qt || '').trim();
  return q ? 'Qt ' + q : '无 Qt';
}

function formatComboHuman(os, arch, compiler, ver, qt, noQt, bt) {
  if (!os && !arch) return '未选目标组合';
  return [displayOs(os), displayArch(arch), compilerBit(compiler, ver), qtBit(qt, noQt), bt || 'Release'].join(' · ');
}

function conanArchToken(v) {
  const e = archEntry(v);
  if (!e) return normalizeArch(v) || String(v || '').trim() || '-';
  // mono 用 Conan 名：aarch64（armv8）→ aarch64
  return e.label.replace(/（.*?）/g, '').trim() || e.id;
}

function formatComboMono(os, arch, compiler, ver, qt, noQt) {
  const c = (compiler || '').trim();
  const v = (ver || '').trim();
  const q = noQt ? '' : (qt || '').trim();
  return (
    'os=' +
    (os || '-') +
    ' arch=' +
    conanArchToken(arch) +
    ' compiler=' +
    (c ? c + (v ? ' ' + v : '') : '-') +
    ' qt=' +
    (noQt ? 'none' : q || 'none')
  );
}

function comboSplit() {
  return !!(
    osSel &&
    archSel &&
    pubOs &&
    pubArch &&
    (osSel !== pubOs ||
      normalizeArch(archSel) !== normalizeArch(pubArch) ||
      (btSel || 'Release') !== (pubBt || 'Release'))
  );
}

function pills(el, items, current, setter) {
  el.innerHTML = items
    .map(([id, label]) => `<button class="chip ${id === current ? 'on' : ''}" data-id="${id}">${label}</button>`)
    .join('');
  el.querySelectorAll('button').forEach((b) => {
    b.onclick = () => {
      setter(b.dataset.id);
      persistPlatform();
      render();
    };
  });
}

function persistPlatform() {
  const payload = {};
  const a = normalizeArch(archSel);
  const pa = normalizeArch(pubArch);
  if (osSel && a && isArchSubmittable(a)) {
    payload.os = osSel;
    payload.arch = a;
  }
  if (btSel) payload.buildType = btSel;
  if (pubOs) payload.publishOs = pubOs;
  if (pa && isArchSubmittable(pa)) payload.publishArch = pa;
  if (pubBt) payload.publishBuildType = pubBt;
  if (Object.keys(payload).length) api.postMessage({ type: 'save-project-quiet', payload });
}

function persistMatch() {
  api.postMessage({
    type: 'save-project-quiet',
    payload: {
      qt: $('p-qt').value.trim(),
      compiler: $('p-compiler').value.trim(),
      compilerVersion: $('p-compiler-ver').value.trim(),
      os: osSel,
      arch: normalizeArch(archSel) || archSel,
      buildType: btSel || 'Release',
      outputFolder: outDir(),
      libDirs: $('p-lib-dirs').value.trim(),
      includeDirs: $('p-include-dirs').value.trim(),
    },
  });
}

function syncPlatformFromProject() {
  const p = project();
  const plat = (p.platform && p.platform.consume) || {};
  osSel = osSel || plat.os || '';
  archSel = normalizeArch(archSel || plat.arch || '') || archSel || plat.arch || '';
  btSel = btSel || plat.build_type || 'Release';
  pubOs = pubOs || (p.platform && p.platform.publish && p.platform.publish.os) || osSel;
  pubArch =
    normalizeArch(pubArch || (p.platform && p.platform.publish && p.platform.publish.arch) || archSel) ||
    pubArch ||
    (p.platform && p.platform.publish && p.platform.publish.arch) ||
    archSel;
  pubBt = pubBt || (p.platform && p.platform.publish && p.platform.publish.build_type) || btSel || 'Release';
}

function renderArchPicker(el, current, onPick, kind) {
  if (!el) return;
  const q = (kind === 'pub' ? pubArchCustomDraft : archCustomDraft) || '';
  const err = kind === 'pub' ? pubArchCustomErr : archCustomErr;
  const query = String(q).trim().toLowerCase();
  const cur = normalizeArch(current);
  const filtered = ARCH_DICT.filter((e) => {
    if (!query) return true;
    const hay = (e.label + ' ' + e.id + ' ' + e.aliases.join(' ')).toLowerCase();
    return hay.includes(query);
  });
  const items =
    filtered
      .map((e) => {
        const on = e.id === cur;
        return `<button type="button" class="arch-item ${on ? 'on' : ''}" data-arch="${e.id}"><span>${esc(e.label)}</span>${on ? '<span class="tag-on">已选</span>' : ''}</button>`;
      })
      .join('') || `<div class="hint" style="padding:10px 12px">无匹配项，可自定义</div>`;
  el.innerHTML =
    `<input id="${kind}-arch-search" placeholder="搜索架构，如 arm / aarch64 / riscv" value="${esc(q)}">` +
    `<div class="arch-list">${items}</div>` +
    `<div class="arch-custom-row"><input id="${kind}-arch-custom" placeholder="自定义 Conan arch…"><button type="button" class="btn" id="${kind}-arch-custom-btn">使用自定义</button></div>` +
    (err
      ? `<p class="arch-err">${esc(err)}</p>`
      : `<p class="hint" style="margin:0">字典含嵌入式常用 arch；armv8 合并为 aarch64，提交映射到插件 arm64</p>`);
  const search = el.querySelector('#' + kind + '-arch-search');
  search.oninput = () => {
    if (kind === 'pub') pubArchCustomDraft = search.value;
    else archCustomDraft = search.value;
    renderArchPicker(el, current, onPick, kind);
  };
  el.querySelectorAll('[data-arch]').forEach((b) => {
    b.onclick = () => {
      if (kind === 'pub') {
        pubArchCustomErr = '';
        pubArchCustomDraft = '';
      } else {
        archCustomErr = '';
        archCustomDraft = '';
      }
      onPick(b.dataset.arch);
    };
  });
  const customInput = el.querySelector('#' + kind + '-arch-custom');
  const customBtn = el.querySelector('#' + kind + '-arch-custom-btn');
  const applyCustom = () => {
    const raw = customInput.value.trim();
    if (!raw || !isLegalConanArch(raw)) {
      const msg = '非法 Conan arch，无法提交';
      if (kind === 'pub') pubArchCustomErr = msg;
      else archCustomErr = msg;
      renderArchPicker(el, current, onPick, kind);
      return;
    }
    const n = normalizeArch(raw);
    if (kind === 'pub') {
      pubArchCustomErr = '';
      pubArchCustomDraft = '';
    } else {
      archCustomErr = '';
      archCustomDraft = '';
    }
    onPick(n);
  };
  customBtn.onclick = applyCustom;
  customInput.onkeydown = (e) => {
    if (e.key === 'Enter') {
      e.preventDefault();
      applyCustom();
    }
  };
}

function paintComboBars() {
  const cHuman = formatComboHuman(
    osSel,
    archSel,
    $('p-compiler').value,
    $('p-compiler-ver').value,
    $('p-qt').value,
    false,
    btSel
  );
  const cMono = formatComboMono(
    osSel,
    archSel,
    $('p-compiler').value,
    $('p-compiler-ver').value,
    $('p-qt').value,
    false
  );
  if ($('consume-combo-title')) $('consume-combo-title').textContent = cHuman;
  if ($('consume-combo-mono')) $('consume-combo-mono').textContent = cMono;
  if ($('consume-combo-edit')) $('consume-combo-edit').hidden = !comboEditOpen;
  if ($('consume-combo-toggle')) $('consume-combo-toggle').textContent = comboEditOpen ? '收起' : '展开修改';
  const pNoQt = $('pub-no-qt') && $('pub-no-qt').checked;
  const pHuman = formatComboHuman(
    pubOs,
    pubArch,
    $('pub-compiler').value,
    $('pub-compiler-ver').value,
    $('pub-qt').value,
    pNoQt,
    pubBt
  );
  const pMono = formatComboMono(
    pubOs,
    pubArch,
    $('pub-compiler').value,
    $('pub-compiler-ver').value,
    $('pub-qt').value,
    pNoQt
  );
  if ($('pub-combo-title')) $('pub-combo-title').textContent = pHuman;
  if ($('pub-combo-mono')) $('pub-combo-mono').textContent = pMono;
  if ($('pub-combo-edit')) $('pub-combo-edit').hidden = !pubComboEditOpen;
  if ($('pub-combo-toggle')) $('pub-combo-toggle').textContent = pubComboEditOpen ? '收起' : '展开修改分项';
  const split = comboSplit();
  if ($('combo-split-warn')) $('combo-split-warn').hidden = !split;
  if ($('pub-split-warn')) $('pub-split-warn').hidden = !split;
  if ($('consume-sync-label')) $('consume-sync-label').textContent = split ? '· 已与发布拆开' : '· 与发布默认同步';
}

function bindPlatformEvents() {
  if ($('consume-combo-toggle')) {
    $('consume-combo-toggle').addEventListener('click', () => {
      comboEditOpen = !comboEditOpen;
      render();
    });
  }
  if ($('pub-combo-toggle')) {
    $('pub-combo-toggle').addEventListener('click', () => {
      pubComboEditOpen = !pubComboEditOpen;
      render();
    });
  }
  ['p-qt', 'p-compiler', 'p-compiler-ver'].forEach((id) => {
    const el = $(id);
    if (el) el.addEventListener('input', () => paintComboBars());
  });
  ['pub-qt', 'pub-compiler', 'pub-compiler-ver'].forEach((id) => {
    const el = $(id);
    if (el) el.addEventListener('input', () => paintComboBars());
  });
}
