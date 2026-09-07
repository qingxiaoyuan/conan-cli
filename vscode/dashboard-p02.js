-q').value.trim()});
      }
    };
    const pills = (el, items, current, setter) => {
      el.innerHTML = items.map(([id, label]) => `<button class="chip ${id===current?'on':''}" data-id="${id}">${label}</button>`).join('');
      el.querySelectorAll('button').forEach((b) => b.onclick = () => { setter(b.dataset.id); persistPlatform(); render(); });
    };
    const persistPlatform = () => {
      const payload = {};
      const a = normalizeArch(archSel);
      const pa = normalizeArch(pubArch);
      if (osSel && a && isArchSubmittable(a)) { payload.os = osSel; payload.arch = a; }
      if (btSel) payload.buildType = btSel;
      if (pubOs) payload.publishOs = pubOs;
      if (pa && isArchSubmittable(pa)) payload.publishArch = pa;
      if (pubBt) payload.publishBuildType = pubBt;
      if (Object.keys(payload).length) api.postMessage({type:'save-project-quiet', payload});
    };
    const outDir = () => ($('p-out').value.trim() || 'conan').replace(/\\/g, '/');
    const persistMatch = () => {
      api.postMessage({type:'save-project-quiet', payload:{
        qt:$('p-qt').value.trim(), compiler:$('p-compiler').value.trim(), compilerVersion:$('p-compiler-ver').value.trim(),
        os:osSel, arch:normalizeArch(archSel)||archSel, buildType: btSel || 'Release', outputFolder: outDir(),
        libDirs:$('p-lib-dirs').value.trim(), includeDirs:$('p-include-dirs').value.trim()
      }});
    };
    const project = () => (state.status && state.status.project) || {};
    const primaryPackage = () => ((project().packages || [])[0] || {});
    const joinDirs = (values) => (values || []).join(', ');
    // 组件清单数据源：优先 status.packages（含 workspace 自动发现与产物探测）；
    // 为空时退回 project.packages[] / 项目名，保持单组件项目的现有体验。
    const packagesList = () => {
      const fromStatus = (state.status && state.status.packages) || [];
      if (fromStatus.length) return fromStatus;
      const p = project();
      const fallback = (name, spec) => ({
        name, version: (spec && spec.version) || '', source: 'declared',
        lib_dirs: (spec && spec.lib_dirs) || [], include_dirs: (spec && spec.include_dirs) || [],
        no_qt: !!(spec && spec.no_qt), has_artifacts: false, has_recipe: false,
      });
      const specs = p.packages || [];
      if (specs.length) return specs.map((s) => fallback(s.name, s));
      if (p.name) return [fallback(p.name, null)];
      return (((state.status && state.status.package_candidates) || [])).map((x) => fallback(x.name, null));
    };
    const g