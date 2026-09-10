// 发布页：组件清单、发布表单、逐包/全部发布。

function pkgRow(info) {
  const prog = pubProgress[info.name];
  let status;
  if (prog && prog.state === 'running') status = '<span class="warn">发布中…</span>';
  else if (prog && prog.state === 'ok')
    status = '<span class="ok">成功</span> <span class="mono detail">' + esc(prog.text) + '</span>';
  else if (prog && prog.state === 'fail') status = '<span class="fail">失败：' + esc(prog.text) + '</span>';
  else status = info.has_artifacts ? '<span class="ok">产物就绪</span>' : '<span class="warn">缺产物</span>';
  const dirs = info.lib_dirs && info.lib_dirs.length ? info.lib_dirs.join(', ') : '默认 lib/、bin/';
  const src = info.source === 'workspace' ? '自动发现' : '已登记';
  const version = info.version ? '@' + info.version : '';
  return (
    `<tr data-name="${esc(info.name)}" class="${info.name === pubSelected ? 'sel-row' : ''}" style="cursor:pointer">` +
    `<td><input type="checkbox" data-check="${esc(info.name)}" ${pubChecked.has(info.name) ? 'checked' : ''} ${publishing ? 'disabled' : ''}></td>` +
    `<td class="mono">${esc(info.name)}${esc(version)}</td>` +
    `<td><span class="tag">${src}</span></td>` +
    `<td class="detail">${esc(dirs)}</td>` +
    `<td>${status}</td>` +
    `<td><button class="btn link" data-pub-one="${esc(info.name)}" ${publishing ? 'disabled' : ''}>发布</button></td>` +
    '</tr>'
  );
}

function renderPackages() {
  const list = packagesList();
  const names = list.map((x) => x.name);
  [...pubChecked].forEach((n) => {
    if (!names.includes(n)) pubChecked.delete(n);
  });
  Object.keys(pubProgress).forEach((n) => {
    if (!names.includes(n)) delete pubProgress[n];
  });
  $('pub-count').textContent = list.length
    ? '共 ' + list.length + ' 个组件' + (pubChecked.size ? '，已勾选 ' + pubChecked.size + ' 个' : '')
    : '';
  $('pub-list').innerHTML =
    list.map(pkgRow).join('') ||
    '<tr><td colspan="6" class="detail">还没有可发布的组件，请在 packages/ 下放置组件或到设置里登记。</td></tr>';
  $('pub-list')
    .querySelectorAll('[data-check]')
    .forEach((c) => {
      c.onclick = (e) => e.stopPropagation();
      c.onchange = () => {
        if (c.checked) pubChecked.add(c.dataset.check);
        else pubChecked.delete(c.dataset.check);
        renderPackages();
      };
    });
  $('pub-list')
    .querySelectorAll('[data-pub-one]')
    .forEach((b) => {
      b.onclick = (e) => {
        e.stopPropagation();
        if (publishing) return;
        selectPackage(b.dataset.pubOne);
        act('publish-preview');
      };
    });
  $('pub-list')
    .querySelectorAll('tr[data-name]')
    .forEach((tr) => {
      tr.onclick = () => {
        if (publishing) return;
        selectPackage(tr.dataset.name);
      };
    });
  const allReady = pubOs && pubArch && $('pub-compiler').value.trim() && $('pub-compiler-ver').value.trim();
  const allBtn = $('pub-all');
  allBtn.disabled = publishing || !list.length || !allReady;
  allBtn.textContent = pubChecked.size ? '发布选中（' + pubChecked.size + '）' : '发布全部';
  $('pub-all-hint').textContent = pubChecked.size
    ? '将按下方表单的平台 / 编译器设置，逐个发布勾选的组件。'
    : '不勾选时一次发布全部组件；勾选后逐个发布，逐行显示进度。';
}

function selectPackage(name) {
  pubSelected = name;
  const p = project();
  const spec = (p.packages || []).find((x) => x.name === name) || {};
  const info = packagesList().find((x) => x.name === name) || {};
  $('pub-name').value = info.name || name;
  delete $('pub-name').dataset.touched;
  const ver = info.version || spec.version || '';
  if (ver) {
    $('pub-version').value = ver;
    $('pub-version').dataset.touched = '1';
  }
  const libDirs = (info.lib_dirs && info.lib_dirs.length ? info.lib_dirs : spec.lib_dirs) || [];
  if (libDirs.length) {
    $('pub-lib-dirs').value = joinDirs(libDirs);
    $('pub-lib-dirs').dataset.touched = '1';
  }
  const incDirs = (info.include_dirs && info.include_dirs.length ? info.include_dirs : spec.include_dirs) || [];
  if (incDirs.length) {
    $('pub-include-dirs').value = joinDirs(incDirs);
    $('pub-include-dirs').dataset.touched = '1';
  }
  $('pub-no-qt').checked = !!(info.no_qt || spec.no_qt);
  $('pub-qt').disabled = $('pub-no-qt').checked;
  if ($('pub-no-qt').checked) {
    $('pub-qt').value = '';
    $('pub-no-qt').dataset.touched = '1';
  } else if (spec.qt_version) {
    $('pub-qt').value = spec.qt_version;
    $('pub-qt').dataset.touched = '1';
  }
  render();
}

function publishAllPayload() {
  return {
    all: true,
    os: pubOs,
    arch: pubArch,
    buildType: pubBt || 'Release',
    compiler: $('pub-compiler').value.trim(),
    compilerVersion: $('pub-compiler-ver').value.trim(),
    channel: $('pub-channel').value.trim() || 'dev',
    note: $('pub-note').value.trim(),
    qt: $('pub-no-qt').checked ? '' : $('pub-qt').value.trim(),
    replace: $('pub-replace').checked,
  };
}

async function startPublishAll() {
  const list = packagesList();
  const checked = [...pubChecked].filter((n) => list.some((x) => x.name === n));
  publishing = true;
  if (!checked.length) {
    list.forEach((x) => {
      pubProgress[x.name] = { state: 'running', text: '' };
    });
    renderPackages();
    api.postMessage({ type: 'publish', payload: publishAllPayload() });
    return;
  }
  loopActive = true;
  for (const name of checked) {
    const info = list.find((x) => x.name === name) || {};
    pubProgress[name] = { state: 'running', text: '' };
    renderPackages();
    const payload = publishAllPayload();
    delete payload.all;
    payload.package = name;
    payload.name = name;
    if (info.version) payload.version = info.version;
    api.postMessage({ type: 'publish', payload });
    await new Promise((resolve) => {
      pendingResolve[name] = resolve;
    });
  }
  loopActive = false;
  publishing = false;
  renderPackages();
}

function handlePublishResult(resp) {
  const data = (resp && resp.data) || {};
  const okText = (ref, replaced) => (ref || '成功') + (replaced ? '（已删旧版 ' + replaced + '）' : '');
  if (Array.isArray(data.results)) {
    data.results.forEach((r) => {
      if (!r || !r.package) return;
      pubProgress[r.package] = r.ok
        ? { state: 'ok', text: okText(r.reference, r.replaced) }
        : { state: 'fail', text: r.error || r.replace_warning || '失败' };
    });
  } else if (data.package) {
    pubProgress[data.package] = resp.ok
      ? { state: 'ok', text: okText(data.reference, data.replaced_reference) }
      : { state: 'fail', text: resp.error || resp.message || '失败' };
  }
  Object.keys(pendingResolve).forEach((k) => {
    pendingResolve[k]();
    delete pendingResolve[k];
  });
  if (!loopActive) publishing = false;
  renderPackages();
}

function publishPayload() {
  const p = (state.status && state.status.project) || {};
  const pkg = (state.status && state.status.package_name) || {};
  return {
    version: $('pub-version').value.trim(),
    channel: $('pub-channel').value.trim() || 'dev',
    note: $('pub-note').value.trim(),
    os: pubOs,
    arch: normalizeArch(pubArch) || pubArch,
    buildType: pubBt || 'Release',
    qt: $('pub-no-qt').checked ? '' : $('pub-qt').value.trim(),
    noQt: $('pub-no-qt').checked,
    compiler: $('pub-compiler').value.trim(),
    compilerVersion: $('pub-compiler-ver').value.trim(),
    libDirs: $('pub-lib-dirs').value.trim() || $('p-lib-dirs').value.trim(),
    includeDirs: $('pub-include-dirs').value.trim() || $('p-include-dirs').value.trim(),
    package: pubSelected,
    name: $('pub-name').value.trim(),
    replace: $('pub-replace').checked,
    _name: $('pub-name').value.trim() || p.name || pkg.name || '',
  };
}

function renderPublish() {
  const p = project();
  const g = global();
  const recipe = (state.status && state.status.recipe) || {};
  const pkg = (state.status && state.status.package_name) || {};
  const pubName = p.name || pkg.name || '';
  const pkgs = packagesList().map((x) => x.name);
  if (pubSelected && !pkgs.includes(pubSelected)) pubSelected = '';
  pubSelected = pubSelected || pkgs[0] || '';
  const currentPkg = pubSelected;
  if (!$('pub-name').dataset.touched) $('pub-name').value = currentPkg || pubName || '';
  $('pub-name-hint').textContent =
    '发布页可改该组件包名。配方写到 .conan-cli/recipes/<包名>/，不会覆盖仓库根的消费配方。';
  if (!$('pub-version').dataset.touched) $('pub-version').value = recipe.version || '';
  if (!$('pub-channel').dataset.touched) $('pub-channel').value = p.channel || 'dev';
  const selectedSpec =
    (p.packages || []).find((x) => x.name === currentPkg) || packagesList().find((x) => x.name === currentPkg) || {};
  if (!$('pub-no-qt').dataset.touched) $('pub-no-qt').checked = !!selectedSpec.no_qt;
  $('pub-qt').disabled = $('pub-no-qt').checked;
  if (!$('pub-qt').dataset.touched)
    $('pub-qt').value = $('pub-no-qt').checked ? '' : selectedSpec.qt_version || p.qt_version || '';
  if (!$('pub-compiler').dataset.touched) $('pub-compiler').value = (p.compiler && p.compiler.id) || '';
  if (!$('pub-compiler-ver').dataset.touched) $('pub-compiler-ver').value = (p.compiler && p.compiler.version) || '';
  const pubVer = $('pub-version').value.trim();
  const liveName = $('pub-name').value.trim() || pubName;
  const prevVer = (selectedSpec && selectedSpec.version) || '';
  $('pub-replace-hint').textContent =
    $('pub-replace').checked && prevVer && prevVer !== pubVer ? '将删除 ' + liveName + '/' + prevVer : '';
  const pubReady =
    liveName &&
    pubVer &&
    pubOs &&
    pubArch &&
    isArchSubmittable(pubArch) &&
    $('pub-compiler').value.trim() &&
    $('pub-compiler-ver').value.trim();
  const qtBit = $('pub-no-qt').checked
    ? '不依赖 Qt'
    : $('pub-qt').value.trim()
      ? 'Qt ' + $('pub-qt').value.trim()
      : 'Qt 跟随项目';
  $('pub-meta').textContent = pubReady
    ? liveName +
      '/' +
      pubVer +
      ' · ' +
      displayOs(pubOs) +
      ' / ' +
      displayArch(pubArch) +
      ' · ' +
      (pubBt || 'Release') +
      ' · ' +
      $('pub-compiler').value.trim() +
      ' ' +
      $('pub-compiler-ver').value.trim() +
      ' · ' +
      qtBit +
      (g.has_password ? ' · 已登录' : ' · 未登录')
    : '请填完包名、版本、发布平台和编译器（Qt 可留空）';
  const hasPy = state.status && state.status.conanfile === 'conanfile.py';
  if (recipe.kind === 'consume')
    $('pub-recipe-hint').textContent = '当前是消费配方。点发布会改成发布配方，并打包本机已编译的库（不会再编译）。';
  else if (!hasPy)
    $('pub-recipe-hint').textContent = '还没有 conanfile.py。点发布会先生成发布配方，再打包本机 lib/ 里已有的库。';
  else
    $('pub-recipe-hint').textContent =
      '点发布会更新配方并打包本机已编译的库和头文件，不会调用 qmake/make。请先按发布页的 Debug/Release 编好。';
  document.querySelectorAll('[data-act="publish-preview"]').forEach((b) => {
    b.disabled = !pubReady;
  });
  if (!$('pub-lib-dirs').dataset.touched) $('pub-lib-dirs').value = $('p-lib-dirs').value;
  if (!$('pub-include-dirs').dataset.touched) $('pub-include-dirs').value = $('p-include-dirs').value;
  pills($('pub-os'), OS, pubOs, (v) => (pubOs = v));
  renderArchPicker(
    $('pub-arch'),
    pubArch,
    (v) => {
      pubArch = v;
      persistPlatform();
      render();
    },
    'pub'
  );
  pills($('pub-bt'), BUILD, pubBt, (v) => (pubBt = v));
  renderPackages();
}

function bindPublishEvents() {
  [
    'pub-version',
    'pub-channel',
    'pub-note',
    'pub-qt',
    'pub-compiler',
    'pub-compiler-ver',
    'pub-lib-dirs',
    'pub-include-dirs',
    'pub-name',
  ].forEach((id) =>
    $(id).addEventListener('input', () => {
      $(id).dataset.touched = '1';
      render();
    })
  );
  $('pub-no-qt').addEventListener('change', () => {
    $('pub-no-qt').dataset.touched = '1';
    $('pub-qt').disabled = $('pub-no-qt').checked;
    if ($('pub-no-qt').checked) $('pub-qt').value = '';
    render();
  });
  $('pub-replace').addEventListener('change', () => render());
  $('modal-cancel').onclick = () => {
    $('modal').classList.remove('show');
    modalAction = null;
  };
  $('modal-ok').onclick = () => {
    $('modal').classList.remove('show');
    const run = modalAction;
    modalAction = null;
    if (run) run();
  };
  $('pub-all').onclick = () => {
    if (publishing) return;
    const list = packagesList();
    if (!list.length) return;
    const checked = [...pubChecked].filter((n) => list.some((x) => x.name === n));
    const targets = checked.length ? checked : list.map((x) => x.name);
    $('modal-text').textContent =
      (checked.length
        ? '将按下方表单的平台设置逐个发布勾选的 ' + checked.length + ' 个组件：'
        : '将按下方表单的平台设置一次发布全部 ' + list.length + ' 个组件：') +
      targets.join('、') +
      ' · ' +
      displayOs(pubOs) +
      ' / ' +
      displayArch(pubArch) +
      ' · ' +
      (pubBt || 'Release') +
      ' · ' +
      $('pub-compiler').value.trim() +
      ' ' +
      $('pub-compiler-ver').value.trim() +
      ($('pub-replace').checked ? ' · 发布后删除各组件的远程旧版本' : '');
    modalAction = startPublishAll;
    $('modal').classList.add('show');
  };
}
