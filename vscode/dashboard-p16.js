      if(name==='scan-fill'){ api.postMessage({type:'scan-fill'}); return; }
      if(name==='recipe-consume'){
        if (state.status && state.status.conanfile === 'conanfile.py') return;
        api.postMessage({type:'recipe-generate', kind:'consume', qt:$('p-qt').value.trim()});
        return;
      }
      if(name==='recipe-publish'){ api.postMessage({type:'recipe-generate', kind:'publish', version:$('pub-version').value.trim(), qt:$('pub-qt').value.trim()}); return; }
      if(name==='install'){ persistMatch(); api.postMessage({type:'install', os:osSel, arch:archSel, buildType: btSel || 'Release', outputFolder: outDir()}); return; }
      if(name==='analyze'){ persistMatch(); api.postMessage({type:'analyze', os:osSel, arch:archSel, buildType: btSel || 'Release'}); return; }
      api.postMessage({type:name, os:osSel, arch:archSel});
    }
    window.addEventListener('message', (e) => {
      const m = e.data || {};
      if (m.type === 'busy') $('busy').textContent = m.label || '';
      if (m.type === 'open-view' && m.view) show(m.view);
      if (m.type === 'catalog') {
        state.catalog = m.catalog || {};
        state.catalogError = m.error || '';
        $('busy').textContent = '';
        fillCatFilters();
        renderCatalog();
      }
      if (m.type === 'scan-fill') {
        const s = m.scan || {};
        const installs = s.qt_installs || [];
        const qt = installs[0] ? (installs[0].short || installs[0].version) : '';
        const compiler = s.compiler || {};
        ['p-qt','pub-qt'].forEach((id) => { if (qt && $(id)) { $(id).value = qt; $(id).dataset.touched = '1'; } });
        ['p-compiler','pub-compiler'].forEach((id) => { if (compiler.id && $(id)) { $(id).value = compiler.id; $(id).dataset.touched = '1'; } });
        ['p-compiler-ver','pub-compiler-ver'].forEach((id) => { if (compiler.version && $(id)) { $(id).value = compiler.version; $(id).dataset.touched = '1'; } });
        $('busy').textContent = '';
        render();
        return;
      }
      if (m.type === 'probe') { state.probe = m.probe || {}; render(); }
      if (m.type === 'publish-result') { handlePublishResult(m.response || {}); }
      if (m.type === 'state') { state = Object.assign(state, m); $('busy').textContent = ''; render(); }
    });
    render();
    api.postMessage({type:'refresh'});
