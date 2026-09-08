    function displayOs(v){ return ({windows:'Windows',linux:'Linux',kylin:'麒麟'}[v]||v||'-'); }
    function normalizeArch(v){
      const key = String(v||'').trim().toLowerCase();
      if (!key) return '';
      if (ARCH_BY_ALIAS[key]) return ARCH_BY_ALIAS[key];
      return key;
    }
    function archEntry(v){
      const id = normalizeArch(v);
      return ARCH_DICT.find((e) => e.id === id) || null;
    }
    function displayArch(v){
      const e = archEntry(v);
      if (e) return e.label;
      const raw = String(v||'').trim();
      return raw || '-';
    }
    function isLegalConanArch(v){
      const raw = String(v||'').trim();
      if (!raw) return false;
      const n = normalizeArch(raw);
      if (ARCH_PLUGIN.has(n)) return true;
      if (ARCH_DICT.some((e) => e.id === n)) return true;
      return !!ARCH_BY_ALIAS[raw.toLowerCase()];
    }
    function isArchSubmittable(v){
      return isLegalConanArch(v);
    }
    function compilerBit(compiler, ver){
      const c = (compiler||'').trim();
      const v = (ver||'').trim();
      if (!c && !v) return '编译器未填';
      return (c || 'compiler') + (v ? (' ' + v) : '');
    }
    function qtBit(qt, noQt){
      if (noQt) return '无 Qt';
      const q = (qt||'').trim();
      return q ? ('Qt ' + q) : '无 Qt';
    }
    function formatComboHuman(os, arch, compiler, ver, qt, noQt, bt){
      if (!os && !arch) return '未选目标组合';
      return [displayOs(os), displayArch(arch), compilerBit(compiler, ver), qtBit(qt, noQt), (bt || 'Release')].join(' · ');
    }
    function conanArchToken(v){
      const e = archEntry(v);
      if (!e) return normalizeArch(v) || String(v||'').trim() || '-';
      // mono 用 Conan 名：aarch64（armv8）→ aarch64
      return e.label.replace(/（.*?）/g, '').trim() || e.id;
    }
    function formatComboMono(os, arch, compiler, ver, qt, noQt){
      const c = (compiler||'').trim();
      const v = (ver||'').trim();
      const q = noQt ? '' : (qt||'').trim();
      return 'os=' + (os||'-') + ' arch=' + conanArchToken(arch)
        + ' compiler=' + (c ? (c + (v ? (' ' + v) : '')) : '-')
        + ' qt=' +