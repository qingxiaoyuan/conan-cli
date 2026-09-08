('input', () => { paintComboBars(); });
    });
    ['pub-qt','pub-compiler','pub-compiler-ver'].forEach((id) => {
      const el = $(id);
      if (el) el.addEventListener('input', () => { paintComboBars(); });
    });
    ['pub-version','pub-channel','pub-note','pub-qt','pub-compiler','pub-compiler-ver','g-url','g-user','g-pass','p-pkg-name','pub-lib-dirs','pub-include-dirs','pub-name'].forEach((id)=>$(id).addEventListener('input',()=>{ $(id).dataset.touched='1'; render(); }));
    $('pub-no-qt').addEventListener('change', () => {
      $('pub-no-qt').dataset.touched = '1';
      $('pub-qt').disabled = $('pub-no-qt').checked;
      if ($('pub-no-qt').checked) $('pub-qt').value = '';
      render();
    });
    $('pub-replace').addEventListener('change', () => render());
    ['p-qt','p-compiler','p-compiler-ver','p-out','p-lib-dirs','p-include-dirs'].forEach((id)=>$(id).addEventListener('change', () => { $(id).dataset.touched='1'; persistMatch(); }));
    $('p-workspaces').addEventListener('input', () => { $('p-workspaces').dataset.touched = '1'; });
    $('cat-q').addEventListener('keydown', (e) => { if (e.key === 'Enter') act('catalog'); });
    $('cat-q').addEventListener('input', () => renderCatalog());
    ['cat-os','cat-arch','cat-compiler','cat-qt','cat-bt'].forEach((id) => {
      const el = $(id);
      if (el) el.addEventListener('change', () => renderCatalog());
    });
    $('modal-cancel').onclick=()=>{ $('modal').classList.remove('show'); modalAction = null; };
    $('modal-ok').onclick=()=>{
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
      $('modal-text').textContent = (checked.length
        ? '将按下方表单的平台设置逐个发布勾选的 ' + checked.length + ' 个组件：'
        : '将按下方表单的平台设置一次发布全部 ' + list.length + ' 个组件：')
        + targets.join('、')
        + ' · ' + displayOs(pubOs) + ' / ' + displayArch(pubArch) + ' · ' + (pubBt || 'Release')
        + ' · ' + $('pub-compiler').value.trim() + ' ' + $('pub-compiler-ver').value.trim()
        + ($('pub-replace').checked ? ' · 发布后删除各组件的远程旧版本' : '');
      modalAction = startPublishAll;
      $('modal').classList.add('show');
    };
    function publishPayload(){
      const p = (state.status && state.status.project) || {};
      const pkg = (state.status && state.status.package_name) || {};
      return {
        version:$('pub-version').value.trim(),
        channel:$('pub-channel')