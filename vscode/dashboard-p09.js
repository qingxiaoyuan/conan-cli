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
        await new Promise((resolve) => { pendingResolve[name] = resolve; });
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
            : { state: 'fail', text: r.error || (r.replace_warning || '失败') };
        });
      } else if (data.package) {
        pubProgress[data.package] = resp.ok
          ? { state: 'ok', text: okText(data.reference, data.replaced_reference) }
          : { state: 'fail', text: resp.error || resp.message || '失败' };
      }
      Object.keys(pendingResolve).forEach((k) => { pendingResolve[k](); delete pendingResolve[k]; });
      if (!loopActive) publishing = false;
      renderPackages();
    }
    function renderCatalog() {
      const packages = (state.catalog && state.catalog.packages) || [];
      const err = state.catalogError || '';
      $('cat-hint').textContent = err || (packages.length ? ('共 ' + packages.length + ' 个组件') : '还没有结果，点查询。');
      $('cat-list').innerHTML = packages.map((pkg) => {
        const open = openPkg === pkg.name;
        const vers = (pkg.versions || []).map((ver) => {
          const ref = pkg.name + '/' + ver;
          return `<div class="ver"><span class="mono">${esc(ref)}</span><button class="btn" data-add="${esc(ref)}">加入依赖</button></div>`;
        }).join('') || '<p class="hint">没有版本信息</p>';
        return `<button class="pkg" data-pkg="${esc(pkg.name)}"><span class="n">${esc(pkg.name)}</span><span class="c">${(pkg.versions||[]).length} 个版本</span></button>${open ? `<div class="vers">${vers}</div>` : ''}`;
      }).join('') || `<div class="hint" style="padding:16px">${esc(err || '仓库里还没有组件，或还没查询。')}<