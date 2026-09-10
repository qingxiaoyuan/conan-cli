// 拉取依赖页：目标组合、install / analyze、消费配方。

function renderDownload() {
  const g = global();
  const p = project();
  pills($('os-pills'), OS, osSel, (v) => (osSel = v));
  renderArchPicker(
    $('arch-pills'),
    archSel,
    (v) => {
      archSel = v;
      persistPlatform();
      render();
    },
    'consume'
  );
  pills($('bt-pills'), BUILD, btSel, (v) => (btSel = v));
  renderDepTable($('dep-rows'));
  paintMissCard($('miss-card-download'), analyzeRows());
  $('combo').textContent =
    osSel && archSel && isArchSubmittable(archSel)
      ? '将按 ' +
        formatComboHuman(
          osSel,
          archSel,
          $('p-compiler').value,
          $('p-compiler-ver').value,
          $('p-qt').value,
          false,
          btSel
        ) +
        ' 拉取到 ' +
        outDir() +
        '/' +
        '（只取仓库二进制，不在本机编译）' +
        (g.has_password ? '' : '。请先到设置里登录仓库')
      : archSel && !isArchSubmittable(archSel)
        ? '架构非法，无法提交'
        : '请先选操作系统和架构（可展开目标组合修改）';
  if (!$('p-qt').dataset.touched) $('p-qt').value = p.qt_version || '';
  if (!$('p-compiler').dataset.touched) $('p-compiler').value = (p.compiler && p.compiler.id) || '';
  if (!$('p-compiler-ver').dataset.touched) $('p-compiler-ver').value = (p.compiler && p.compiler.version) || '';
  if (!$('p-out').dataset.touched) $('p-out').value = p.output_folder || 'conan';
  document.querySelectorAll('[data-act="install"]').forEach((b) => {
    b.disabled = !(osSel && archSel && isArchSubmittable(archSel));
  });
  document.querySelectorAll('[data-act="analyze"]').forEach((b) => {
    b.disabled = !!(archSel && !isArchSubmittable(archSel));
  });
  const hasPy = state.status && state.status.conanfile === 'conanfile.py';
  const recipeBtn = $('recipe-consume');
  if (recipeBtn) {
    recipeBtn.textContent = hasPy ? '配方就绪' : '生成配方';
    recipeBtn.classList.toggle('ready', !!hasPy);
    recipeBtn.disabled = !!hasPy;
    recipeBtn.title = hasPy ? '已有 conanfile.py' : '生成消费用 conanfile.py';
  }
}

function bindDownloadEvents() {
  ['p-qt', 'p-compiler', 'p-compiler-ver', 'p-out'].forEach((id) =>
    $(id).addEventListener('change', () => {
      $(id).dataset.touched = '1';
      persistMatch();
    })
  );
}
