import { request } from '@umijs/max';

// 设计器（模板管理 + SQL 预览）API，对应后端 /api/admin/*。

export type TemplateSummary = {
  code: string;
  name: string;
  version: number;
  group: string;
  has_submit: boolean;
};

export type PreviewColumn = {
  name: string;
  db_type: string;
  type: string; // text | int | money | date
};

export type SqlPreviewResult = {
  columns: PreviewColumn[];
  rows: unknown[][];
};

function unwrap<T>(p: Promise<unknown>): Promise<T> {
  return p.then((body) => (body as { data: T }).data);
}

export async function listTemplates() {
  return unwrap<TemplateSummary[]>(request('/api/admin/templates', { method: 'GET' }));
}

export async function getTemplate(code: string) {
  return unwrap<{ def: unknown; raw: string }>(
    request(`/api/admin/templates/${code}`, { method: 'GET' }),
  );
}

export async function saveTemplate(code: string, yaml: string) {
  return unwrap<{ saved: string }>(
    request('/api/admin/templates', { method: 'POST', data: { code, yaml } }),
  );
}

export async function deleteTemplate(code: string) {
  return unwrap<{ deleted: string }>(
    request(`/api/admin/templates/${code}`, { method: 'DELETE' }),
  );
}

export async function reloadTemplates() {
  return unwrap<{ reloaded: boolean }>(request('/api/admin/reload', { method: 'POST' }));
}

export async function sqlPreview(
  source: string,
  sql: string,
  paramsDef: { key: string; label: string; type: string; required?: boolean }[],
  values: Record<string, string>,
) {
  return unwrap<SqlPreviewResult>(
    request('/api/admin/sql-preview', {
      method: 'POST',
      data: { source, sql, params_def: paramsDef, values },
    }),
  );
}
