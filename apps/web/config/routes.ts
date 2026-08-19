export default [
  { path: '/', redirect: '/reports' },
  { path: '/reports', name: 'reports', component: './ReportList' },
  { path: '/reports/:code', name: 'report-form', hideInMenu: true, component: './ReportForm' },
];
