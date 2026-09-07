 (noQt ? 'none' : (q || 'none'));
    }
    function comboSplit(){
      return !!(osSel && archSel && pubOs && pubArch && (osSel !== pubOs || normalizeArch(archSel) !== normalizeArch(pubArch) || (btSel||'Release') !== (pubBt||'Release')));
    }
    function renderArchPicker(el, current, onPick, kind){
      if (!el) return;
      const q = (kind === 'pub' ? pubArchCustomDraft : archCustomDraft) || '';
      const err = kind === 'pub' ? pubArchCustomErr : archCustomErr;
      const query = String(q).trim().toLowerCase();
      const cur = normalizeArch(current);
      const filtered = ARCH_DICT.filter((e) => {
        if (!query) return true;
        const hay = (e.label + ' ' + e.id + ' ' + e.aliases.join(' ')).toLowerCase();
        return hay.includes(query);
      });
      const items = filtered.map((e) => {
        const on = e.id === cur;
        return `<button type="button" class="arch-item ${on?'on':''}" data-arch="${e.id}"><span>${esc(e.label)}</span>${on?'<span class="tag-on">已选</span>':''}</button>`;
      }).join('') || `<div class="hint" style="padding:10px 12px">无匹配项，可自定义</div>`;
      el.innerHTML = `<input id="${kind}-arch-search" placeholder="搜索架构，如 arm / aarch64 / riscv" value="${esc(q)}">`
        + `<div class="arch-list">${items}</div>`
        + `<div class="arch-custom-row"><input id="${kind}-arch-custom" placeholder="自定义 Conan arch…"><button type="button" class="btn" id="${kind}-arch-custom-btn">使用自定义</button></div>`
        + (err ? `<p class="arch-err">${esc(err)}</p>` : `<p class="hint" style="margin:0">字典含嵌入式常用 arch；armv8 合并为 aarch64，提交映射到插件 arm64</p>`);
      const search = el.querySelector('#' + kind + '-arch-search');
      search.oninput = () => {
        if (kind === 'pub') pubArchCustomDraft = search.value;
        else archCustomDraft = search.value;
        renderArchPicker(el, current, onPick, kind);
      };
      el.querySelectorAll('[data-arch]').forEach((b) => b.onclick = () => {
        if (kind === 'pub') { pubArchCustomErr = ''; pubArchCustomDraft = ''; }
        else { archCustomErr = ''; archCustomDraft = ''; }
        onPick(b.dataset.arch);
      });
      const customInput = el.querySelector('#' + kind + '-arch-custom');
      const customBtn = el.querySelector('#' + kind + '-arch-custom-btn');
      const applyCustom = () => {
        const raw = customInput.value.trim();
        if (!raw || !isLegalConanArch(raw)) {
          const msg = '非法 Conan arch，无法提交';
          if (kind === 'pub') pubArchCustomE