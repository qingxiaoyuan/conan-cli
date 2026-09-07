.value.trim() || 'dev', note:$('pub-note').value.trim(),
        os:pubOs, arch:normalizeArch(pubArch)||pubArch, buildType: pubBt || 'Release',         qt:$('pub-no-qt').checked ? '' : $('pub-qt').value.trim(),
        noQt: $('pub-no-qt').checked,
        compiler:$('pub-compiler').value.trim(), compilerVersion:$('pub-compiler-ver').value.trim(),
        libDirs: $('pub-lib-dirs').value.trim() || $('p-lib-dirs').value.trim(),
        includeDirs: $('pub-include-dirs').value.trim() || $('p-include-dirs').value.trim(),
        package: pubSelected,
        name: $('pub-name').value.trim(),
        replace: $('pub-replace').checked,
        _name: $('pub-name').value.trim() || p.name || pkg.name || ''
      };
    }
    function act(name){
      if(name==='save-pkg-name'){
        const next = $('p-pkg-name').value.trim();
        const cur = ((state.status && state.status.project) || {}).name || '';
        if (!next) return;
        if (next !== cur && !window.confirm('改包名会变成仓库里的新包，旧引用全部失效。确定改成 ' + next + '？')) return;
        api.postMessage({type:'save-project', payload:{name:next, libDirs:$('p-lib-dirs').value.trim(), includeDirs:$('p-include-dirs').value.trim(), workspaces:$('p-workspaces').value.trim()}});
        return;
      }
      if(name==='publish-preview'){
        const pld = publishPayload();
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
      if(name==='catalog'){ api.postMessage({type:'catalog', query:$('cat-q').value.tri