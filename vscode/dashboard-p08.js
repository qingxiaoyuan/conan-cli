lue.trim();
      const allBtn = $('pub-all');
      allBtn.disabled = publishing || !list.length || !allReady;
      allBtn.textContent = pubChecked.size ? ('发布选中（' + pubChecked.size + '）') : '发布全部';
      $('pub-all-hint').textContent = pubChecked.size
        ? '将按下方表单的平台 / 编译器设置，逐个发布勾选的组件。'
        : '不勾选时一次发布全部组件；勾选后逐个发布，逐行显示进度。';
    }
    function selectPackage(name) {
      pubSelected = name;
      const p = project();
      const spec = ((p.packages || []).find((x) => x.name === name)) || {};
      const info = (packagesList().find((x) => x.name === name)) || {};
      $('pub-name').value = info.name || name;
      delete $('pub-name').dataset.touched;
      const ver = info.version || spec.version || '';
      if (ver) { $('pub-version').value = ver; $('pub-version').dataset.touched = '1'; }
      const libDirs = (info.lib_dirs && info.lib_dirs.length ? info.lib_dirs : spec.lib_dirs) || [];
      if (libDirs.length) { $('pub-lib-dirs').value = joinDirs(libDirs); $('pub-lib-dirs').dataset.touched = '1'; }
      const incDirs = (info.include_dirs && info.include_dirs.length ? info.include_dirs : spec.include_dirs) || [];
      if (incDirs.length) { $('pub-include-dirs').value = joinDirs(incDirs); $('pub-include-dirs').dataset.touched = '1'; }
      $('pub-no-qt').checked = !!(info.no_qt || spec.no_qt);
      $('pub-qt').disabled = $('pub-no-qt').checked;
      if ($('pub-no-qt').checked) { $('pub-qt').value = ''; $('pub-qt').dataset.touched = '1'; }
      else if (spec.qt_version) { $('pub-qt').value = spec.qt_version; $('pub-qt').dataset.touched = '1'; }
      render();
    }
    function publishAllPayload() {
      return {
        all: true,
        os: pubOs, arch: pubArch, buildType: pubBt || 'Release',
        compiler: $('pub-compiler').value.trim(), compilerVersion: $('pub-compiler-ver').value.trim(),
        channel: $('pub-channel').value.trim() || 'dev', note: $('pub-note').value.trim(),
        qt: $('pub-no-qt').checked ? '' : $('pub-qt').value.trim(),
        replace: $('pub-replace').checked,
      };
    }
    async function startPublishAll() {
      const list = packagesList();
      const checked = [...pubChecked].filter((n) => list.some((x) => x.name === n));
      publishing = true;
      if (!checked.length) {
        list.forEach((x) => { pubProgress[x.name] = { state: 'running', text: '' }; });
        renderPackages();
        api.postMessage({ type: 'publish', payload: publishAllPayload() });
        return;
      }
      