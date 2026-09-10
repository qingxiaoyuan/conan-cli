// 导航、总渲染、命令分发、与 extension 的消息。

function show(view) {
  if (view === 'overview') view = 'download';
  document
    .querySelectorAll('[data-section]')
    .forEach((el) => el.classList.toggle('active', el.dataset.section === view));
  document
    .querySelectorAll('.nav-btn[data-view]')
    .forEach((el) => el.classList.toggle('active', el.dataset.view === view));
  if (view === 'catalog' && !((state.catalog && state.catalog.packages) || []).length) {
    api.postMessage(catalogQuery());
  }
}

function renderChrome() {
  const p = project();
  const g = global();
  const probeNow = state.probe || {};
  $('workspace').textContent = '工作区 / ' + (p.name || '未初始化');
  $('project-name').textContent = p.name || '选择目标平台后拉取依赖';
  $('login-badge').textContent = probeNow.ok ? '已连接' : g.has_password ? '已登录' : '未登录';
  $('login-dot').className = 'dot' + (probeNow.ok || g.has_password ? '' : ' warn');
  $('platform-chip').textContent =
    osSel || archSel
      ? '目标组合 · ' + displayOs(osSel) + ' / ' + displayArch(archSel) + ' · ' + (btSel || 'Release')
      : '未选目标组合';
}

function render() {
  syncPlatformFromProject();
  renderChrome();
  renderDownload();
  renderDeps();
  renderSettings();
  renderPublish();
  renderDoctor();
  renderCatalog();
  paintComboBars();
}

function act(name) {
  if (name === 'save-pkg-name') {
    const next = $('p-pkg-name').value.trim();
    const cur = ((state.status && state.status.project) || {}).name || '';
    if (!next) return;
    if (next !== cur && !window.confirm('改包名会变成仓库里的新包，旧引用全部失效。确定改成 ' + next + '？')) return;
    api.postMessage({
      type: 'save-project',
      payload: {
        name: next,
        libDirs: $('p-lib-dirs').value.trim(),
        includeDirs: $('p-include-dirs').value.trim(),
        workspaces: $('p-workspaces').value.trim(),
      },
    });
    return;
  }
  if (name === 'publish-preview') {
    const pld = publishPayload();
    if (!(pld._name && pld.version && pld.os && pld.arch && pld.compiler && pld.compilerVersion)) return;
    $('modal-text').textContent =
      '将打包本机已编译的 ' +
      pld._name +
      '/' +
      pld.version +
      '（不再编译）· ' +
      displayOs(pld.os) +
      ' / ' +
      displayArch(pld.arch) +
      ' · ' +
      (pld.buildType || 'Release') +
      ' · ' +
      pld.compiler +
      ' ' +
      pld.compilerVersion +
      (pld.noQt ? ' · 不依赖 Qt' : pld.qt ? ' · Qt ' + pld.qt : '') +
      (pld.replace ? ' · 发布后删除远程旧版本' : '');
    modalAction = () => {
      publishing = true;
      pubProgress[pubSelected] = { state: 'running', text: '' };
      renderPackages();
      api.postMessage({ type: 'publish', payload: publishPayload() });
    };
    $('modal').classList.add('show');
    return;
  }
  if (name === 'add') {
    api.postMessage({ type: 'add' });
    return;
  }
  if (name === 'save-global') {
    api.postMessage({
      type: 'save-global',
      payload: {
        name: $('g-name').value || 'nexus',
        url: $('g-url').value,
        username: $('g-user').value,
        password: $('g-pass').value,
      },
    });
    return;
  }
  if (name === 'install' && !(osSel && archSel)) {
    return;
  }
  if (name === 'catalog') {
    api.postMessage(catalogQuery());
    return;
  }
  if (name === 'scan-fill') {
    api.postMessage({ type: 'scan-fill' });
    return;
  }
  if (name === 'recipe-consume') {
    if (state.status && state.status.conanfile === 'conanfile.py') return;
    api.postMessage({ type: 'recipe-generate', kind: 'consume', qt: $('p-qt').value.trim() });
    return;
  }
  if (name === 'recipe-publish') {
    api.postMessage({
      type: 'recipe-generate',
      kind: 'publish',
      version: $('pub-version').value.trim(),
      qt: $('pub-qt').value.trim(),
    });
    return;
  }
  if (name === 'install') {
    persistMatch();
    api.postMessage({
      type: 'install',
      os: osSel,
      arch: archSel,
      buildType: btSel || 'Release',
      outputFolder: outDir(),
    });
    return;
  }
  if (name === 'analyze') {
    persistMatch();
    api.postMessage({ type: 'analyze', os: osSel, arch: archSel, buildType: btSel || 'Release' });
    return;
  }
  api.postMessage({ type: name, os: osSel, arch: archSel });
}

function applyScanFill(scan) {
  const s = scan || {};
  const installs = s.qt_installs || [];
  const qt = installs[0] ? installs[0].short || installs[0].version : '';
  const compiler = s.compiler || {};
  ['p-qt', 'pub-qt'].forEach((id) => {
    if (qt && $(id)) {
      $(id).value = qt;
      $(id).dataset.touched = '1';
    }
  });
  ['p-compiler', 'pub-compiler'].forEach((id) => {
    if (compiler.id && $(id)) {
      $(id).value = compiler.id;
      $(id).dataset.touched = '1';
    }
  });
  ['p-compiler-ver', 'pub-compiler-ver'].forEach((id) => {
    if (compiler.version && $(id)) {
      $(id).value = compiler.version;
      $(id).dataset.touched = '1';
    }
  });
}

function bindDashboardEvents() {
  document.querySelectorAll('[data-view]').forEach((b) => b.addEventListener('click', () => show(b.dataset.view)));
  document.querySelectorAll('[data-act]').forEach((b) => b.addEventListener('click', () => act(b.dataset.act)));
  bindPlatformEvents();
  bindDownloadEvents();
  bindCatalogEvents();
  bindPublishEvents();
  bindSettingsEvents();
  window.addEventListener('message', (e) => {
    const m = e.data || {};
    if (m.type === 'busy') $('busy').textContent = m.label || '';
    if (m.type === 'open-view' && m.view) show(m.view);
    if (m.type === 'catalog') {
      state.catalog = m.catalog || {};
      state.catalogError = m.error || '';
      $('busy').textContent = '';
      fillCatFilters();
      renderCatalog();
    }
    if (m.type === 'scan-fill') {
      applyScanFill(m.scan);
      $('busy').textContent = '';
      render();
      return;
    }
    if (m.type === 'probe') {
      state.probe = m.probe || {};
      render();
    }
    if (m.type === 'publish-result') {
      handlePublishResult(m.response || {});
    }
    if (m.type === 'state') {
      state = Object.assign(state, m);
      $('busy').textContent = '';
      render();
    }
  });
}

bindDashboardEvents();
render();
api.postMessage({ type: 'refresh' });
