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
    function compilerLabel(b) {
      const c = ((b && b.compiler) || '').trim();
      const v = ((b && b.compiler_version) || '').trim();
      if (!c && !v) return '';
      return (c || 'compiler') + (v ? (' ' + v) : '');
    }
    function catalogFilters() {
      return {
        q: (($('cat-q') && $('cat-q').value) || '').trim().toLowerCase(),
        os: ($('cat-os') && $('cat-os').value) || '',
        arch: ($('cat-arch') && $('cat-arch').value) || '',
        compiler: ($('cat-compiler') && $('cat-compiler').value) || '',
        qt: ($('cat-qt') && $('cat-qt').value) || '',
        bt: ($('cat-bt') && $('cat-bt').value) || '',
      };
    }
    function binaryMatches(rec, f) {
      if (f.os && rec.os !== f.os) return false;
      if (f.arch && rec.arch !== f.arch) return false;
      if (f.bt && rec.build_type !== f.bt) return false;
      if (f.compiler && compilerLabel(rec) !== f.compiler && rec.compiler !== f.compiler) return false;
      if (f.qt === 'none' && !rec.no_qt) return false;
      if (f.qt && f.qt !== 'none' && rec.qt_version !== f.qt) return false;
      return true;
    }
    function catalogPackages() {
      const packages = (state.catalog && state.catalog.packages) || [];
      const f = catalogFilters();
      const hasBinFilter = !!(f.os || f.arch || f.compiler || f.qt || f.bt);
      return packages.map((pkg) => {
        if (f.q && !(pkg.name || '').toLowerCase().includes(f.q)) return null;
        const bins = (pkg.binaries || []).filter((b) => binaryMatches(b, f));
        const versions = pkg.versions || [];
        if (hasBinFilter && !bins.length) return null;
        return { pkg, bins, versions };
      }).filter(Boolean);
    }
    function binaryCells(r) {
      const hasBinary = !!(r.os || r.arch || r.compiler || r.build_type || r.qt_version || r.no_qt);
      const plat = (r.os || r.arch) ? (displayOs(r.os) + ' / ' + displayArch(r.arch)) : '-';
      const qt = !hasBinary ? '-' : (r.no_qt || !r.qt_version ? '无 Qt' : ('Qt ' + r.qt_version));
      return { plat, comp: compilerLabel(r) || '-', qt, bt: r.build_type || '-' };
    }
    function fillCatFilters() {
      const packages = (state.catalog && state.catalog.packages) || [];
      const compilers = {};
      const qts = {};
      packages.forEach((pkg) => (pkg.binaries || []).forEach((b) => {
        const cl = compilerLabel(b);
        if (cl) compilers[cl] = 1;
        if (b.no_qt) qts['none'] = 1;
        else if (b.qt_version) qts[b.qt_version] = 1;
      }));
      const fill = (id, extra, values) => {
        const sel = $(id);
        if (!sel) return;
        const cur = sel.value;
        const keep = extra.slice();
        Object.keys(values).sort().forEach((v) => {
          if (v === 'none') return;
          keep.push([v, id === 'cat-qt' ? ('Qt ' + v) : v]);
        });
        sel.innerHTML = keep.map(([v, l]) => `<option value="${esc(v)}">${esc(l)}</option>`).join('');
        if (keep.some((x) => x[0] === cur)) sel.value = cur;
      };
      fill('cat-compiler', [['', '全部编译器']], compilers);
      fill('cat-qt', [['', '全部 Qt'], ['none', '无 Qt']], qts);
    }
    function renderCatalog() {
      const err = state.catalogError || '';
      const all = (state.catalog && state.catalog.packages) || [];
      const rows = catalogPackages();
      const binTotal = rows.reduce((n, r) => n + r.bins.length, 0);
      const missing = all.length && !binTotal;
      $('cat-hint').textContent = err || (all.length
        ? ('共 ' + rows.length + ' 个组件' + (binTotal ? ('，' + binTotal + ' 条制品。点组件展开查看平台 / 编译器 / Qt / 构建。') : '。点组件展开查看版本。'))
        : '还没有结果，点查询。');
      if (missing && !err) $('cat-hint').textContent += ' 未能读取制品平台信息，请确认已登录远程仓库。';
      if (!all.length) {
        $('cat-list').innerHTML = `<div class="hint" style="padding:16px">${esc(err || '仓库里还没有组件，或还没查询。')}</div>`;
        return;
      }
      if (!rows.length) {
        $('cat-list').innerHTML = `<div class="hint" style="padding:16px">${esc(err || '没有匹配的组件，试试改筛选条件。')}</div>`;
        return;
      }
      $('cat-list').innerHTML = rows.map((row) => {
        const name = row.pkg.name;
        const open = openPkg === name;
        const summary = row.bins.length
          ? (row.versions.length + ' 个版本 · ' + row.bins.length + ' 条制品')
          : ((row.versions.length || 0) + ' 个版本');
        const detailRows = row.bins.length
          ? row.bins.map((b) => {
            const rec = Object.assign({ name, version: b.version, reference: name + '/' + b.version }, b);
            const c = binaryCells(rec);
            return `<tr><td class="mono">${esc(rec.version || '-')}</td><td>${esc(c.plat)}</td><td>${esc(c.comp)}</td><td>${esc(c.qt)}</td><td>${esc(c.bt)}</td><td><button class="btn" data-add="${esc(rec.reference)}">加入依赖</button></td></tr>`;
          }).join('')
          : (row.versions.map((ver) => `<tr><td class="mono">${esc(ver)}</td><td>-</td><td>-</td><td>-</td><td>-</td><td><button class="btn" data-add="${esc(name + '/' + ver)}">加入依赖</button></td></tr>`).join('') || '<tr><td colspan="6" class="detail">没有版本信息</td></tr>');
        const detail = open ? `<div class="vers"><table><thead><tr><th>版本</th><th>平台</th><th>编译器</th><th>Qt</th><th>构建</th><th></th></tr></thead><tbody>${detailRows}</tbody></table></div>` : '';
        return `<button class="pkg ${open?'open':''}" data-pkg="${esc(name)}"><span class="chev">${open?'▾':'▸'}</span><span class="n">${esc(name)}</span><span class="c">${esc(summary)}</span></button>${detail}`;
      }).join('');
      $('cat-list').querySelectorAll('[data-pkg]').forEach((b) => {
        b.onclick = () => { openPkg = openPkg === b.dataset.pkg ? '' : b.dataset.pkg; renderCatalog(); };
      });
      $('cat-list').querySelectorAll('[data-add]').forEach((b) => {
        b.onclick = (e) => { e.stopPropagation(); api.postMessage({type:'add-ref', ref: b.dataset.add}); };
      });
    }