import type { ReportInfo } from '@jyx-bi/rpt-types';

/** 参数默认值：month 取当前月，date 取当月 1 日 */
export function defaultParams(info: ReportInfo): Record<string, string> {
  const out: Record<string, string> = {};
  const now = new Date();
  const ym = `${now.getFullYear()}-${String(now.getMonth() + 1).padStart(2, '0')}`;
  info.params.forEach((p) => {
    out[p.key] = p.type === 'month' ? ym : p.type === 'date' ? `${ym}-01` : '';
  });
  return out;
}

/** 填报页 URL（带默认参数） */
export function reportUrl(code: string, params: Record<string, string>) {
  return `/reports/${code}?${new URLSearchParams(params).toString()}`;
}

/** 从 axios 错误里取服务端返回的具体错误信息（{ok:false,error:{code,message}}） */
export function errMsg(e: any, fallback: string): string {
  return e?.response?.data?.error?.message ?? e?.error?.message ?? fallback;
}
