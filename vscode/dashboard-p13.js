('pub-compiler-ver').value, $('pub-qt').value, pNoQt);
      if ($('pub-combo-title')) $('pub-combo-title').textContent = pHuman;
      if ($('pub-combo-mono')) $('pub-combo-mono').textContent = pMono;
      if ($('pub-combo-edit')) $('pub-combo-edit').hidden = !pubComboEditOpen;
      if ($('pub-combo-toggle')) $('pub-combo-toggle').textContent = pubComboEditOpen ? '收起' : '展开修改分项';
      const split = comboSplit();
      if ($('combo-split-warn')) $('combo-split-warn').hidden = !split;
      if ($('pub-split-warn')) $('pub-split-warn').hidden = !split;
      if ($('consume-sync-label')) $('consume-sync-label').textContent = split ? '· 已与发布拆开' : '· 与发布默认同步';
    }
    const CHECKS = {
      conan: { title: 'Conan 是否可用', ok: '本机可以运行 Conan 2。', fail: '找不到 Conan，请先安装 Conan 2。' },
      project_config: { title: '项目配置', ok: '已有项目配置。', fail: '选平台并拉取依赖时会自动创建。' },
      conanfile: { title: '配方文件', ok: '已有 conanfile。', fail: '缺少配方。拉取依赖时会生成最简 conanfile.txt。' },
      global_remote: { title: '仓库登录', ok: 'Nexus 地址和账号已保存。', fail: '请到设置里填写地址、用户名并保存登录。' },
      platform: { title: '目标平台', ok: '已选择制品要跑的系统和架构。', fail: '到「拉取依赖」页点选操作系统和架构即可。' },
    };
    function checkMeta(name) {
      return CHECKS[name] || { title: name, ok: '检查通过。', fail: '检查未通过。' };
    }
    function friendlyDetail(name, detail) {
      const text = String(detail || '').trim();
      if (name === 'conan' && /Conan version/i.test(text)) return text.replace(/^Conan version/i, '版本');
      if (name === 'project_config' && text.includes('project.yaml')) return '路径：' + text;
      return text;
    }
    function statusLabel(v){ return ({found:'已找到',missing_binary:'无匹配二进制',missing_package:'无此包',missing_version:'无此版本',mismatch:'不一致',unknown:'未查询'}[v]||v); }
    function esc(v){ return String(v??'').replace(/[&<>"']/g,(c)=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#039;'}[c])); }
    document.querySelectorAll('[data-view]').forEach((b)=>b.addEventListener('click', () => show(b.dataset.view)));
    document.querySelectorAll('[data-act]').forEach((b)=>b.addEventListener('click', () => act(b.dataset.act)));
    if ($('consume-combo-toggle')) $('consume-combo-toggle').addEventListener('click', () => { comboEditOpen = !comboEditOpen; render(); });
    if ($('pub-combo-toggle')) $('pub-combo-toggle').addEventListener('click', () => { pubComboEditOpen = !pubComboEditOpen; render(); });
    ['p-qt','p-compiler','p-compiler-ver'].forEach((id) => {
      const el = $(id);
      if (el) el.addEventListener