lobal = () => (state.status && state.status.global) || {nexus:{}};
    function isMissingStatus(s) {
      return s === 'missing_binary' || s === 'missing_package' || s === 'missing_version';
    }
    function depRow(r) {
      const detail = isMissingStatus(r.status) ? ((r.detail || '') + (r.detail ? ' · ' : '') + '见下方缺制品卡片') : (r.detail || '');
      return `<tr class="${r.status==='found'||r.status==='unknown'?'':'warn-row'}"><td class="mono">${esc(r.reference)}</td><td class="${r.status==='found'?'ok':'warn'}">${esc(statusLabel(r.status))}</td><td class="detail">${esc(detail)}</td></tr>`;
    }
    function render() {
      const p = project();
      const g = global();
      const plat = (p.platform && p.platform.consume) || {};
      osSel = osSel || plat.os || '';
      archSel = normalizeArch(archSel || plat.arch || '') || (archSel || plat.arch || '');
      btSel = btSel || plat.build_type || 'Release';
      pubOs = pubOs || (p.platform && p.platform.publish && p.platform.publish.os) || osSel;
      pubArch = normalizeArch(pubArch || (p.platform && p.platform.publish && p.platform.publish.arch) || archSel) || (pubArch || (p.platform && p.platform.publish && p.platform.publish.arch) || archSel);
      pubBt = pubBt || (p.platform && p.platform.publish && p.platform.publish.build_type) || btSel || 'Release';
      $('workspace').textContent = '工作区 / ' + (p.name || '未初始化');
      $('project-name').textContent = p.name || '选择目标平台后拉取依赖';
      const nexus = g.nexus || {};
      const probeNow = state.probe || {};
      $('login-badge').textContent = probeNow.ok ? '已连接' : (g.has_password ? '已登录' : '未登录');
      $('login-dot').className = 'dot' + (probeNow.ok || g.has_password ? '' : ' warn');
      $('platform-chip').textContent = (osSel || archSel) ? ('目标组合 · ' + displayOs(osSel) + ' / ' + displayArch(archSel) + ' · ' + (btSel || 'Release')) : '未选目标组合';
      const rows = (state.analyze && state.analyze.dependencies) || [];
      const empty = '<tr><td colspan="3" class="detail">选好平台后点「检查依赖」或「拉取依赖」</td></tr>';
      $('dep-rows').innerHTML = rows.map(depRow).join('') || empty;
      $('dep-rows-full').innerHTML = rows.map(depRow).join('') || empty;
      pills($('os-pills'), OS, osSel, (v) => osSel = v);
      renderArchPicker($('arch-pills'), archSel, (v) => { archSel = v; persistPlatform(); render(); }, 'consume');
      pills($('bt-pills'), BUILD, btSel, (v) => btSel = v);
      pills($('pub-os'), OS, pubOs, (v) => pubOs = v);
      renderArchPick