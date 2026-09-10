// 设置页：包名、产物目录、仓库登录。

function renderSettings() {
  const p = project();
  const g = global();
  const nexus = g.nexus || {};
  const pkg = (state.status && state.status.package_name) || {};
  const pubName = p.name || pkg.name || '';
  if (!$('p-pkg-name').dataset.touched) $('p-pkg-name').value = pubName;
  const srcHint =
    { qmake: 'qmake TARGET', cmake: 'CMake', include: '头文件目录', recipe: '已有配方', directory: '目录名' }[
      pkg.source
    ] ||
    pkg.source ||
    '';
  if (pkg.name && srcHint && pkg.source !== 'directory') {
    $('pkg-name-hint').textContent = '当前按' + srcHint + '识别。工程名与组件包名已分开；多组件请在发布页选择。';
  } else {
    $('pkg-name-hint').textContent = '工程名可与组件包名不同。多组件在项目 packages[] 里分别命名。';
  }
  if (!$('g-url').dataset.touched) $('g-url').value = nexus.url || '';
  if (!$('g-user').dataset.touched) $('g-user').value = nexus.username || '';
  $('g-name').value = nexus.name || 'nexus';
  const probe = state.probe || {};
  const bits = [];
  if (nexus.url) bits.push('地址 ' + nexus.url);
  if (nexus.username) bits.push('用户 ' + nexus.username);
  if (g.has_password) bits.push('密码已保存');
  if (probe.ok === true) bits.push('仓库可达');
  if (probe.ok === false) bits.push('连接失败' + (probe.message ? '：' + probe.message : ''));
  $('g-hint').textContent = bits.length ? bits.join(' · ') : '还没有仓库配置，填一次后保存即可。';
  if (!$('p-lib-dirs').dataset.touched) $('p-lib-dirs').value = joinDirs(primaryPackage().lib_dirs);
  if (!$('p-include-dirs').dataset.touched) $('p-include-dirs').value = joinDirs(primaryPackage().include_dirs);
  if (!$('p-workspaces').dataset.touched) $('p-workspaces').value = (p.workspaces || []).join(', ');
}

function bindSettingsEvents() {
  ['g-url', 'g-user', 'g-pass', 'p-pkg-name'].forEach((id) =>
    $(id).addEventListener('input', () => {
      $(id).dataset.touched = '1';
      render();
    })
  );
  $('p-workspaces').addEventListener('input', () => {
    $('p-workspaces').dataset.touched = '1';
  });
  ['p-lib-dirs', 'p-include-dirs'].forEach((id) =>
    $(id).addEventListener('change', () => {
      $(id).dataset.touched = '1';
      persistMatch();
    })
  );
}
