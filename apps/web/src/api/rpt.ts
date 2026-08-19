import { request } from '@umijs/max';

import type {
  DraftRequest,
  GridSpec,
  ImportReport,
  Issue,
  ReportInfo,
  RowPayload,
} from '@jyx-bi/rpt-types';

function qs(params: Record<string, string>) {
  const p = new URLSearchParams();
  Object.entries(params).forEach(([k, v]) => {
    if (v !== undefined && v !== '') p.set(k, v);
  });
  return p.toString();
}

// 后端 200 响应统一是 { ok, data } 信封，而 request() 原样返回整个响应体
// （dataField 解包只作用于 useRequest hook，不作用于直接调用的 request()），
// 这里统一提取 .data，与调用处声明的类型对齐
async function unwrap<T>(p: Promise<unknown>): Promise<T> {
  const body = (await p) as { data: T };
  return body.data;
}

export async function listReports() {
  return unwrap<ReportInfo[]>(request('/api/reports', { method: 'GET' }));
}

export async function getGrid(code: string, params: Record<string, string>) {
  return unwrap<GridSpec>(
    request(`/api/reports/${code}/grid?${qs(params)}`, { method: 'GET' }),
  );
}

export async function saveDraft(
  code: string,
  params: Record<string, string>,
  body: DraftRequest,
) {
  return unwrap<unknown>(
    request(`/api/reports/${code}/draft?${qs(params)}`, {
      method: 'PUT',
      data: body,
    }),
  );
}

export async function validateGrid(
  code: string,
  params: Record<string, string>,
  rows: RowPayload[],
) {
  return unwrap<{ ok: boolean; issues: Issue[] }>(
    request(`/api/reports/${code}/validate?${qs(params)}`, {
      method: 'POST',
      data: { rows },
    }),
  );
}

export async function submit(
  code: string,
  params: Record<string, string>,
  body: DraftRequest,
) {
  return unwrap<unknown>(
    request(`/api/reports/${code}/submit?${qs(params)}`, {
      method: 'POST',
      data: body,
    }),
  );
}

export async function withdraw(code: string, params: Record<string, string>) {
  return unwrap<unknown>(
    request(`/api/reports/${code}/withdraw?${qs(params)}`, { method: 'POST' }),
  );
}

export function exportUrl(code: string, params: Record<string, string>) {
  return `/api/reports/${code}/export.xlsx?${qs(params)}`;
}

export async function importFile(
  code: string,
  params: Record<string, string>,
  file: File,
) {
  const form = new FormData();
  form.append('file', file);
  return unwrap<ImportReport>(
    request(`/api/reports/${code}/import?${qs(params)}`, {
      method: 'POST',
      data: form,
      requestType: 'form',
    }),
  );
}

export async function confirmImport(
  code: string,
  params: Record<string, string>,
  jobId: number,
) {
  return unwrap<unknown>(
    request(`/api/reports/${code}/import/${jobId}/confirm?${qs(params)}`, {
      method: 'POST',
    }),
  );
}
