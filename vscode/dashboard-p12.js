rr = msg; else archCustomErr = msg;
          renderArchPicker(el, current, onPick, kind);
          return;
        }
        const n = normalizeArch(raw);
        if (kind === 'pub') { pubArchCustomErr = ''; pubArchCustomDraft = ''; }
        else { archCustomErr = ''; archCustomDraft = ''; }
        onPick(n);
      };
      customBtn.onclick = applyCustom;
      customInput.onkeydown = (e) => { if (e.key === 'Enter') { e.preventDefault(); applyCustom(); } };
    }
    function paintMissCard(el, rows){
      if (!el) return;
      const miss = (rows || []).filter((r) => isMissingStatus(r.status));
      if (!miss.length) { el.hidden = true; el.innerHTML = ''; return; }
      const refs = miss.map((r) => r.reference).filter(Boolean);
      const human = formatComboHuman(osSel, archSel, $('p-compiler').value, $('p-compiler-ver').value, $('p-qt').value, false, btSel);
      el.hidden = false;
      el.innerHTML = `<h3>仓库没有这套制品</h3>`
        + `<p>当前组合：<strong>${esc(human)}</strong></p>`
        + `<p>缺失 <span class="mono">${esc(refs.join('、'))}</span>。请联系你们的制品负责人；不要在本机编译。</p>`
        + `<div class="actions" style="margin-top:12px"><button type="button" class="btn primary" data-fix-combo>改组合</button></div>`
        + `<p class="hint">不会执行 --build=missing。原始 Conan 输出可在诊断页展开。</p>`;
      el.querySelector('[data-fix-combo]').onclick = () => {
        comboEditOpen = true;
        show('download');
        render();
        const bar = $('consume-combo-edit');
        if (bar) bar.scrollIntoView({behavior:'smooth', block:'start'});
      };
    }
    function paintComboBars(){
      const cHuman = formatComboHuman(osSel, archSel, $('p-compiler').value, $('p-compiler-ver').value, $('p-qt').value, false, btSel);
      const cMono = formatComboMono(osSel, archSel, $('p-compiler').value, $('p-compiler-ver').value, $('p-qt').value, false);
      if ($('consume-combo-title')) $('consume-combo-title').textContent = cHuman;
      if ($('consume-combo-mono')) $('consume-combo-mono').textContent = cMono;
      if ($('consume-combo-edit')) $('consume-combo-edit').hidden = !comboEditOpen;
      if ($('consume-combo-toggle')) $('consume-combo-toggle').textContent = comboEditOpen ? '收起' : '展开修改';
      const pNoQt = $('pub-no-qt') && $('pub-no-qt').checked;
      const pHuman = formatComboHuman(pubOs, pubArch, $('pub-compiler').value, $('pub-compiler-ver').value, $('pub-qt').value, pNoQt, pubBt);
      const pMono = formatComboMono(pubOs, pubArch, $('pub-compiler').value, $