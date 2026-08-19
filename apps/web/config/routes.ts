export default [
  { path: '/', redirect: '/portal' },
  { path: '/portal', name: '报表门户', component: './Portal' },
  { path: '/reports', name: '报表清单', component: './ReportList' },
  { path: '/designer', name: '报表设计', component: './Designer' },
  { path: '/reports/:code', name: 'report-form', hideInMenu: true, component: './ReportForm' },
  // 裸模式：门户页 iframe 内嵌填报页，不带 ProLayout 外壳
  { path: '/embed/:code', name: 'embed', hideInMenu: true, layout: false, component: './ReportForm' },
];
