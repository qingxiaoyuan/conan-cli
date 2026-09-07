 (p.compiler && p.compiler.version) || '';
      const pubVer = $('pub-version').value.trim();
      const liveName = $('pub-name').value.trim() || pubName;
      const prevVer = (selectedSpec && selectedSpec.version) || '';
      $('pub-replace-hint').textContent = ($('pub-replace').checked && prevVer && prevVer !== pubVer) ? '将删除 ' + liveName + '/' + prevVer : '';
      const pubReady = liveName && pubVer && pubOs && pubArch && isArchSubmittable(pubArch) && $('pub-compiler').value.trim() && $('pub-compiler-ver').value.trim();
      const qtBit = $('pub-no-qt').checked ? '不依赖 Qt' : ($('pub-qt').value.trim() ? 'Qt ' + $('pub-qt').value.trim() : 'Qt 跟随项目');
      $('pub-meta').textContent = pubReady
        ? (liveName + '/' + pubVer + ' · ' + displayOs(pubOs) + ' / ' + displayArch(pubArch) + ' · ' + (pubBt || 'Release') + ' · ' + $('pub-compiler').value.trim() + ' ' + $('pub-compiler-ver').value.trim() + ' · ' + qtBit + (g.has_password ? ' · 已登录' : ' · 未登录'))
        : '请填完包名、版本、发布平台和编译器（Qt 可留空）';
      const hasPy = state.status && state.status.conanfile === 'conanfile.py';
      if (recipe.kind === 'consume') $('pub-recipe-hint').textContent = '当前是消费配方。点发布会改成发布配方，并打包本机已编译的库（不会再编译）。';
      else if (!hasPy) $('pub-recipe-hint').textContent = '还没有 conanfile.py。点发布会先生成发布配方，再打包本机 lib/ 里已有的库。';
      else $('pub-recipe-hint').textContent = '点发布会更新配方并打包本机已编译的库和头文件，不会调用 qmake/make。请先按发布页的 Debug/Release 编好。';
      document.querySelectorAll('[data-act="publish-preview"]').forEach((b) => { b.disabled = !pubReady; });
      if (!$('g-url').dataset.touched) $('g-url').value = nexus.url || '';
      if (!$('g-user').dataset.touched) $('g-user').value = nexus.username || '';
      $('g-name').value = nexus.name || 'nexus';
      const probe = state.probe || {};
      const bits = [];
      if (nexus.url) bits.push('地址 ' + nexus.url);
      if (nexus.username) bits.push('用户 ' + nexus.username);
      if (g.has_password) bits.push('密码已保存');
      if (probe.ok === true) bits.push('仓库可达');
      if (probe.ok === false) bits.push('连接失败' + (probe.message ? '：' + probe.message : ''));
      $('g-hint').textContent = bits.length ? bits.join(' · ') : '还没有仓库配置，填一次后保存即可。';
      if (!$('p-qt').dataset.touched) $('p-qt').value = p.qt_version || '';
      if (!$('p-compiler').dataset.touched) $('p-compiler').value = (p.compiler && p.compiler.id) || '';
      if (!$('p-compiler-ver').dataset.touched) $('p-compiler-ver').value = (p.compiler && p.compiler.version) || '';
      if (!