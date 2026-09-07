rog.text) + '</span>';
      else if (prog && prog.state === 'fail') status = '<span class="fail">失败：' + esc(prog.text) + '</span>';
      else status = info.has_artifacts ? '<span class="ok">产物就绪</span>' : '<span class="warn">缺产物</span>';
      const dirs = (info.lib_dirs && info.lib_dirs.length) ? info.lib_dirs.join(', ') : '默认 lib/、bin/';
      const src = info.source === 'workspace' ? '自动发现' : '已登记';
      const version = info.version ? '@' + info.version : '';
      return `<tr data-name="${esc(info.name)}" class="${info.name===pubSelected?'sel-row':''}" style="cursor:pointer">`
        + `<td><input type="checkbox" data-check="${esc(info.name)}" ${pubChecked.has(info.name)?'checked':''} ${publishing?'disabled':''}></td>`
        + `<td class="mono">${esc(info.name)}${esc(version)}</td>`
        + `<td><span class="tag">${src}</span></td>`
        + `<td class="detail">${esc(dirs)}</td>`
        + `<td>${status}</td>`
        + `<td><button class="btn link" data-pub-one="${esc(info.name)}" ${publishing?'disabled':''}>发布</button></td>`
        + '</tr>';
    }
    function renderPackages() {
      const list = packagesList();
      const names = list.map((x) => x.name);
      [...pubChecked].forEach((n) => { if (!names.includes(n)) pubChecked.delete(n); });
      Object.keys(pubProgress).forEach((n) => { if (!names.includes(n)) delete pubProgress[n]; });
      $('pub-count').textContent = list.length
        ? ('共 ' + list.length + ' 个组件' + (pubChecked.size ? '，已勾选 ' + pubChecked.size + ' 个' : ''))
        : '';
      $('pub-list').innerHTML = list.map(pkgRow).join('')
        || '<tr><td colspan="6" class="detail">还没有可发布的组件，请在 packages/ 下放置组件或到设置里登记。</td></tr>';
      $('pub-list').querySelectorAll('[data-check]').forEach((c) => {
        c.onclick = (e) => e.stopPropagation();
        c.onchange = () => {
          if (c.checked) pubChecked.add(c.dataset.check); else pubChecked.delete(c.dataset.check);
          renderPackages();
        };
      });
      $('pub-list').querySelectorAll('[data-pub-one]').forEach((b) => b.onclick = (e) => {
        e.stopPropagation();
        if (publishing) return;
        selectPackage(b.dataset.pubOne);
        act('publish-preview');
      });
      $('pub-list').querySelectorAll('tr[data-name]').forEach((tr) => tr.onclick = () => {
        if (publishing) return;
        selectPackage(tr.dataset.name);
      });
      const allReady = pubOs && pubArch && $('pub-compiler').value.trim() && $('pub-compiler-ver').va