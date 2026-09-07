shPayload();
        if (!(pld._name && pld.version && pld.os && pld.arch && pld.compiler && pld.compilerVersion)) return;
        $('modal-text').textContent = '将打包本机已编译的 ' + pld._name + '/' + pld.version + '（不再编译）· ' + displayOs(pld.os) + ' / ' + displayArch(pld.arch) + ' · ' + (pld.buildType || 'Release') + ' · ' + pld.compiler + ' ' + pld.compilerVersion + (pld.noQt ? ' · 不依赖 Qt' : (pld.qt ? ' · Qt ' + pld.qt : '')) + (pld.replace ? ' · 发布后删除远程旧版本' : '');
        modalAction = () => {
          publishing = true;
          pubProgress[pubSelected] = { state: 'running', text: '' };
          renderPackages();
          api.postMessage({type:'publish', payload: publishPayload()});
        };
        $('modal').classList.add('show');
        return;
      }
      if(name==='add'){ api.postMessage({type:'add'}); return; }
      if(name==='save-global'){ api.postMessage({type:'save-global', payload:{name:$('g-name').value||'nexus', url:$('g-url').value, username:$('g-user').value, password:$('g-pass').value}}); return; }
      if(name==='install' && !(osSel && archSel)){ return; }
      if(name==='catalog'){ api.postMessage(catalogQuery()); return; }
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
        fillCatChannels();
        renderCatalog();
      }
      if (m.type === 'scan-fill') {
        const s = 