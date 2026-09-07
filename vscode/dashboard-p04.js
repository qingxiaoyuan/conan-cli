er($('pub-arch'), pubArch, (v) => { pubArch = v; persistPlatform(); render(); }, 'pub');
      pills($('pub-bt'), BUILD, pubBt, (v) => pubBt = v);
      paintMissCard($('miss-card-download'), rows);
      paintMissCard($('miss-card-deps'), rows);
      $('combo').textContent = (osSel && archSel && isArchSubmittable(archSel))
        ? ('将按 ' + formatComboHuman(osSel, archSel, $('p-compiler').value, $('p-compiler-ver').value, $('p-qt').value, false, btSel) + ' 拉取到 ' + outDir() + '/' + '（只取仓库二进制，不在本机编译）' + (g.has_password ? '' : '。请先到设置里登录仓库'))
        : (archSel && !isArchSubmittable(archSel) ? '架构非法，无法提交' : '请先选操作系统和架构（可展开目标组合修改）');
      const recipe = (state.status && state.status.recipe) || {};
      const pkg = (state.status && state.status.package_name) || {};
      const pubName = p.name || pkg.name || '';
      const pkgs = packagesList().map((x) => x.name);
      if (pubSelected && !pkgs.includes(pubSelected)) pubSelected = '';
      pubSelected = pubSelected || pkgs[0] || '';
      const currentPkg = pubSelected;
      if (!$('pub-name').dataset.touched) $('pub-name').value = currentPkg || pubName || '';
      if (!$('p-pkg-name').dataset.touched) $('p-pkg-name').value = pubName;
      const srcHint = {qmake:'qmake TARGET', cmake:'CMake', include:'头文件目录', recipe:'已有配方', directory:'目录名'}[pkg.source] || pkg.source || '';
      $('pub-name-hint').textContent = '发布页可改该组件包名。配方写到 .conan-cli/recipes/<包名>/，不会覆盖仓库根的消费配方。';
      if (pkg.name && srcHint && pkg.source !== 'directory') {
        $('pkg-name-hint').textContent = '当前按' + srcHint + '识别。工程名与组件包名已分开；多组件请在发布页选择。';
      } else {
        $('pkg-name-hint').textContent = '工程名可与组件包名不同。多组件在项目 packages[] 里分别命名。';
      }
      if (!$('pub-version').dataset.touched) $('pub-version').value = recipe.version || '';
      if (!$('pub-channel').dataset.touched) $('pub-channel').value = p.channel || 'dev';
      const selectedSpec = ((p.packages || []).find((x) => x.name === currentPkg)) || (packagesList().find((x) => x.name === currentPkg)) || {};
      if (!$('pub-no-qt').dataset.touched) $('pub-no-qt').checked = !!selectedSpec.no_qt;
      $('pub-qt').disabled = $('pub-no-qt').checked;
      if (!$('pub-qt').dataset.touched) $('pub-qt').value = $('pub-no-qt').checked ? '' : (selectedSpec.qt_version || p.qt_version || '');
      if (!$('pub-compiler').dataset.touched) $('pub-compiler').value = (p.compiler && p.compiler.id) || '';
      if (!$('pub-compiler-ver').dataset.touched) $('pub-compiler-ver').value =