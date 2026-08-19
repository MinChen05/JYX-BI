import { request } from '@umijs/max';

import type {
  DraftRequest,
  GridSpec,
  ImportReport,
  Issue,
  ReportInfo,
  RowPayload,
} from '@kingdee-rpt/rpt-types';

function qs(params: Record<string, string>) {
  const p = new URLSearchParams();
  Object.entries(params).forEach(([k, v]) => {
    if (v !== undefined && v !== '') p.set(k, v);
  });
  return p.toString();
}

export async function listReports() {
  return request<ReportInfo[]>('/api/reports', { method: 'GET' });
}

export async function getGrid(code: string, params: Record<string, string>) {
  return request<GridSpec>(`/api/reports/${code}/grid?${qs(params)}`, {
    method: 'GET',
  });
}

export async function saveDraft(
  code: string,
  params: Record<string, string>,
  body: DraftRequest,
) {
  return request(`/api/reports/${code}/draft?${qs(params)}`, {
    method: 'PUT',
    data: body,
  });
}

export async function validateGrid(
  code: string,
  params: Record<string, string>,
  rows: RowPayload[],
) {
  return request<{ ok: boolean; issues: Issue[] }>(
    `/api/reports/${code}/validate?${qs(params)}`,
    { method: 'POST', data: { rows } },
  );
}

export async function submit(
  code: string,
  params: Record<string, string>,
  body: DraftRequest,
) {
  return request(`/api/reports/${code}/submit?${qs(params)}`, {
    method: 'POST',
    data: body,
  });
}

export async function withdraw(code: string, params: Record<string, string>) {
  return request(`/api/reports/${code}/withdraw?${qs(params)}`, {
    method: 'POST',
  });
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
  return request<ImportReport>(`/api/reports/${code}/import?${qs(params)}`, {
    method: 'POST',
    data: form,
    requestType: 'form',
  });
}

export async function confirmImport(
  code: string,
  params: Record<string, string>,
  jobId: number,
) {
  return request(`/api/reports/${code}/import/${jobId}/confirm?${qs(params)}`, {
    method: 'POST',
  });
}
