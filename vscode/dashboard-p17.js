m.scan || {};
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
      if (m.type === 'state') { state = Object.assign(state, m); $('busy').textContent = ''; fillCatChannels(); render(); }
    });
    render();
    api.postMessage({type:'refresh'});
