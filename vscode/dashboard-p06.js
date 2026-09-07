$('p-out').dataset.touched) $('p-out').value = p.output_folder || 'conan';
      if (!$('p-lib-dirs').dataset.touched) $('p-lib-dirs').value = joinDirs(primaryPackage().lib_dirs);
      if (!$('p-include-dirs').dataset.touched) $('p-include-dirs').value = joinDirs(primaryPackage().include_dirs);
      if (!$('p-workspaces').dataset.touched) $('p-workspaces').value = (p.workspaces || []).join(', ');
      if (!$('pub-lib-dirs').dataset.touched) $('pub-lib-dirs').value = $('p-lib-dirs').value;
      if (!$('pub-include-dirs').dataset.touched) $('pub-include-dirs').value = $('p-include-dirs').value;
      paintComboBars();
      document.querySelectorAll('[data-act="install"]').forEach((b) => { b.disabled = !(osSel && archSel && isArchSubmittable(archSel)); });
      document.querySelectorAll('[data-act="analyze"]').forEach((b) => { b.disabled = !!(archSel && !isArchSubmittable(archSel)); });
      const recipeBtn = $('recipe-consume');
      if (recipeBtn) {
        recipeBtn.textContent = hasPy ? '配方就绪' : '生成配方';
        recipeBtn.classList.toggle('ready', !!hasPy);
        recipeBtn.disabled = !!hasPy;
        recipeBtn.title = hasPy ? '已有 conanfile.py' : '生成消费用 conanfile.py';
      }
      const checks = ((state.doctor && state.doctor.checks) || []).filter((c) => !SKIP_CHECKS[c.name]);
      const failed = checks.filter((c) => !c.ok).length;
      $('doctor-summary').innerHTML = checks.length
        ? (failed ? `<strong class="warn">${failed} 项需要处理</strong><span class="hint">共 ${checks.length} 项</span>` : `<strong class="ok">就绪</strong><span class="hint">共 ${checks.length} 项</span>`)
        : '';
      $('doctor-list').innerHTML = checks.map((c) => {
        const meta = checkMeta(c.name);
        return `<div class="check"><div class="check-icon ${c.ok?'ok':'fail'}">${c.ok?'✓':'!'}</div><div><div class="check-title">${esc(meta.title)}</div><div class="check-desc">${esc(c.ok ? meta.ok : meta.fail)}</div></div><div class="check-result">${esc(friendlyDetail(c.name, c.detail))}</div></div>`;
      }).join('') || '<div class="hint" style="padding:16px">还没有诊断结果。</div>';
      $('raw-out').textContent = state.raw || '';
      renderPackages();
      renderCatalog();
    }
    function pkgRow(info) {
      const prog = pubProgress[info.name];
      let status;
      if (prog && prog.state === 'running') status = '<span class="warn">发布中…</span>';
      else if (prog && prog.state === 'ok') status = '<span class="ok">成功</span> <span class="mono detail">' + esc(p