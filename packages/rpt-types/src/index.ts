// kingdee-rpt 前后端共享类型（与服务端 GridSpec/ImportReport 契约一致）

export interface ColumnSpec {
  key: string;
  label: string;
  type: 'text' | 'money' | 'int' | 'date' | 'month' | 'auto';
  readonly?: boolean;
  formula?: string;
  width?: number;
}

export interface RowSpec {
  row_key: string;
  cells: Record<string, any>;
}

export interface InstanceInfo {
  id: number;
  status: number; // 0 草稿 1 已提交
  updated_at: string;
}

export interface RowOps {
  add: boolean;
  delete: boolean;
}

export interface GridSpec {
  report: string;
  name: string;
  version: number;
  params: Record<string, string>;
  instance: InstanceInfo | null;
  columns: ColumnSpec[];
  rows: RowSpec[];
  row_ops: RowOps;
  number_format: string;
}

export interface ParamDef {
  key: string;
  label: string;
  type: 'month' | 'date' | 'text';
  required?: boolean;
}

export interface InstanceBrief {
  params: Record<string, string>;
  status: number;
  updated_at: string;
  updated_by: string;
  submitted_at: string | null;
}

export interface ReportInfo {
  code: string;
  name: string;
  version: number;
  params: ParamDef[];
  instances: InstanceBrief[];
}

export interface Issue {
  row_key: string;
  row_idx: number;
  col?: string;
  value?: any;
  message: string;
}

export interface CellDiff {
  row_key: string;
  col: string;
  old: any;
  new: any;
}

export interface ImportReport {
  job_id: number;
  status: string;
  errors: Issue[];
  changed: number;
  unmatched: string[];
  cells: CellDiff[];
}

export interface RowPayload {
  row_key: string;
  cells: Record<string, any>;
}

export interface DraftRequest {
  expected_updated_at: string | null;
  rows: RowPayload[];
}

export const STATUS_TEXT: Record<number, string> = { 0: '草稿', 1: '已提交' };
