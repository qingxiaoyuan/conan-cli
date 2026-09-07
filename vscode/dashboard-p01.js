const api = acquireVsCodeApi();
    const OS = [['windows','Windows'],['linux','Linux'],['kylin','麒麟']];
    // 展示用 Conan arch 字典；提交值映射到现插件 x86/x64/arm/arm64（其余原样，须为合法 Conan arch）
    const ARCH_DICT = [
      {id:'x86', label:'x86', aliases:['x86','i386','i686']},
      {id:'x64', label:'x86_64', aliases:['x86_64','x64','amd64','x86-64']},
      {id:'arm', label:'armv7', aliases:['armv7','arm','arm32','armhf','armel']},
      {id:'arm64', label:'aarch64（armv8）', aliases:['aarch64','armv8','arm64']},
      {id:'ppc32', label:'ppc32', aliases:['ppc32','ppc']},
      {id:'ppc64', label:'ppc64', aliases:['ppc64']},
      {id:'ppc64le', label:'ppc64le', aliases:['ppc64le']},
      {id:'s390x', label:'s390x', aliases:['s390x']},
      {id:'mips', label:'mips', aliases:['mips']},
      {id:'mips64', label:'mips64', aliases:['mips64']},
      {id:'sparc', label:'sparc', aliases:['sparc']},
      {id:'sparcv9', label:'sparcv9', aliases:['sparcv9']},
      {id:'riscv64', label:'riscv64', aliases:['riscv64']},
    ];
    const ARCH_PLUGIN = new Set(['x86','x64','arm','arm64']);
    const ARCH_BY_ALIAS = (() => {
      const m = {};
      ARCH_DICT.forEach((e) => { e.aliases.forEach((a) => { m[a.toLowerCase()] = e.id; }); m[e.id] = e.id; });
      m['armv8'] = 'arm64';
      return m;
    })();
    let comboEditOpen = false, pubComboEditOpen = false;
    let archCustomDraft = '', pubArchCustomDraft = '';
    let archCustomErr = '', pubArchCustomErr = '';
    const BUILD = [['Debug','Debug'],['Release','Release']];
    const SKIP_CHECKS = { profiles:1, remotes:1, configured_remote:1, manifest_dependencies:1 };
    let state = {status:{}, doctor:{}, analyze:{}, catalog:{}};
    let osSel = '', archSel = '', btSel = '', pubOs = '', pubArch = '', pubBt = '', openPkg = '';
    let pubSelected = '', publishing = false, loopActive = false, modalAction = null;
    const pubChecked = new Set();
    const pubProgress = {};
    const pendingResolve = {};
    const $ = (id) => document.getElementById(id);
    const show = (view) => {
      if (view === 'overview') view = 'download';
      document.querySelectorAll('[data-section]').forEach((el) => el.classList.toggle('active', el.dataset.section === view));
      document.querySelectorAll('.nav-btn[data-view]').forEach((el) => el.classList.toggle('active', el.dataset.view === view));
      if (view === 'catalog' && !((state.catalog && state.catalog.packages) || []).length) {
        api.postMessage({type:'catalog', query: $('cat