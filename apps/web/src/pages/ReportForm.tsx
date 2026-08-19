import {
  Alert,
  Card,
  Drawer,
  message,
  Modal,
  Space,
  Spin,
  Table,
  Tag,
  Typography,
} from 'antd';
import React, { useCallback, useEffect, useRef, useState } from 'react';
import dayjs from 'dayjs';

import {
  exportUrl,
  getGrid,
  listReports,
  saveDraft,
  submit,
  validateGrid,
  withdraw,
} from '../api/rpt';
import HotGrid, { type GridHandle } from '../components/HotGrid';
import ImportDialog from '../components/ImportDialog';
import ParamBar from '../components/ParamBar';
import Toolbar from '../components/Toolbar';
import type {
  DraftRequest,
  GridSpec,
  Issue,
  ReportInfo,
  RowPayload,
} from '@jyx-bi/rpt-types';
import { STATUS_TEXT } from '@jyx-bi/rpt-types';

/**
 * 通用填报页：完全由 GridSpec 驱动，不感知具体报表。
 */
const ReportForm: React.FC = () => {
  // 路由参数直接解析 URL（页面跳转用 window.location，无需 router hooks）
  const code = window.location.pathname.split('/').filter(Boolean).pop() ?? '';
  const [spec, setSpec] = useState<GridSpec | null>(null);
  const [reportMeta, setReportMeta] = useState<ReportInfo | null>(null);
  const [paramValues, setParamValues] = useState<Record<string, string>>({});
  const [rowKeys, setRowKeys] = useState<string[]>([]);
  const [gridData, setGridData] = useState<any[][]>([]);
  const [issues, setIssues] = useState<Issue[] | null>(null);
  const [importOpen, setImportOpen] = useState(false);
  const [loading, setLoading] = useState(true);
  const [paramDefs, setParamDefs] = useState<ReportInfo['params']>([]);
  const [msgApi, msgContextHolder] = message.useMessage();
  const hotRef = useRef<GridHandle>(null);

  const params = paramValues;
  const locked = spec?.instance?.status === 1;

  const collect = useCallback((): RowPayload[] => {
    if (!spec) return [];
    const data = hotRef.current?.getData() ?? [];
    return rowKeys.map((rk, i) => {
      const cells: Record<string, any> = {};
      spec.columns.forEach((c, ci) => {
        if (c.type === 'auto' || c.readonly) return;
        const v = data[i]?.[ci];
        if (v !== undefined && v !== null && v !== '') cells[c.key] = v;
      });
      return { row_key: rk, cells };
    });
  }, [spec, rowKeys]);

  const toDraftRequest = (rows: RowPayload[]): DraftRequest => ({
    expected_updated_at: spec?.instance?.updated_at ?? null,
    rows,
  });

  const load = useCallback(async () => {
    if (!code || Object.keys(params).length === 0) return;
    setLoading(true);
    try {
      const g: GridSpec = (await getGrid(code, params)) as GridSpec;
      setSpec(g);
      setReportMeta({
        code: g.report,
        name: g.name,
        version: g.version,
        params: [],
        instances: [],
      });
      setRowKeys(g.rows.map((r) => r.row_key));
      setGridData(
        g.rows.map((r) => g.columns.map((c) => r.cells[c.key] ?? null)),
      );
    } catch (e: any) {
      msgApi.error(e?.error?.message ?? '加载失败');
    } finally {
      setLoading(false);
    }
  }, [code, params, msgApi]);

  useEffect(() => {
    // 从 URL query 初始化参数
    if (!code) return;
    setParamValues(Object.fromEntries(new URLSearchParams(window.location.search)));
    // 取模板参数定义（label/type），供参数栏渲染
    listReports().then((all: ReportInfo[]) => {
      const meta = all.find((r: ReportInfo) => r.code === code);
      if (meta) setParamDefs(meta.params);
    });
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [code]);

  useEffect(() => {
    if (Object.keys(params).length > 0) load();
  }, [params, load]);

  const onDraft = async () => {
    try {
      await saveDraft(code, params, toDraftRequest(collect()));
      msgApi.success('草稿已保存');
      load();
    } catch (e: any) {
      msgApi.error(e?.error?.message ?? '保存失败');
    }
  };

  const onValidate = async () => {
    try {
      const r = await validateGrid(code, params, collect());
      setIssues(r.issues);
    } catch (e: any) {
      msgApi.error(e?.error?.message ?? '校验失败');
    }
  };

  const onSubmit = () => {
    Modal.confirm({
      title: '提交报表',
      content: '提交后将执行全量校验并写入 Doris，同时触发推送。确认提交？',
      onOk: async () => {
        try {
          await submit(code, params, toDraftRequest(collect()));
          msgApi.success('提交成功');
          load();
        } catch (e: any) {
          msgApi.error(e?.error?.message ?? '提交失败');
        }
      },
    });
  };

  const onWithdraw = async () => {
    try {
      await withdraw(code, params);
      msgApi.success('已撤回，可继续编辑');
      load();
    } catch (e: any) {
      msgApi.error(e?.error?.message ?? '撤回失败');
    }
  };

  const onAddRow = () => {
    const idx = hotRef.current?.insertRowEnd() ?? 0;
    setRowKeys((ks) => {
      const next = [...ks];
      next.splice(idx, 0, `n${Date.now().toString(36)}`);
      return next;
    });
    setGridData(hotRef.current?.getData() ?? []);
  };

  const onDeleteRow = () => {
    const sel = hotRef.current?.selectedRows() ?? [];
    if (sel.length === 0) {
      msgApi.warning('请先选中要删除的行');
      return;
    }
    hotRef.current?.deleteSelected();
    setRowKeys((ks) => {
      const next = [...ks];
      [...sel].sort((a, b) => b - a).forEach((i) => next.splice(i, 1));
      return next;
    });
    setGridData(hotRef.current?.getData() ?? []);
  };

  const onGridChange = (data: any[][], keys: string[]) => {
    setGridData(data);
    setRowKeys(keys);
  };

  return (
    <>
      {msgContextHolder}
      <Spin spinning={loading}>
        <Card
          title={
            <Space>
              <span>{spec?.name ?? code}</span>
              {spec?.instance && (
                <Tag color={locked ? 'green' : 'orange'}>
                  {STATUS_TEXT[spec.instance.status]}
                  {spec.instance.updated_at &&
                    ` · ${dayjs(spec.instance.updated_at).format('MM-DD HH:mm')}`}
                </Tag>
              )}
            </Space>
          }
          extra={
            <ParamBar
              params={paramDefs}
              values={params}
              onChange={(v) => setParamValues(v)}
            />
          }
        >
          <Toolbar
            locked={!!locked}
            editable={spec?.editable ?? false}
            rowOps={spec?.row_ops ?? { add: false, delete: false }}
            onDraft={onDraft}
            onValidate={onValidate}
            onAddRow={onAddRow}
            onDeleteRow={onDeleteRow}
            onExport={() => window.open(exportUrl(code, params))}
            onImport={() => setImportOpen(true)}
            onSubmit={onSubmit}
            onWithdraw={onWithdraw}
            onPrint={() => window.print()}
          />
          <div className="rpt-grid-holder">
            <HotGrid
              ref={hotRef}
              columns={spec?.columns ?? []}
              rowKeys={rowKeys}
              initialData={gridData}
              rowOps={spec?.row_ops ?? { add: false, delete: false }}
              disabled={!!locked}
              onData={onGridChange}
            />
          </div>
        </Card>
      </Spin>

      <Drawer
        title="数据校验结果"
        open={issues !== null}
        onClose={() => setIssues(null)}
        width={640}
      >
        {issues && issues.length === 0 ? (
          <Alert type="success" message="校验通过，无错误" />
        ) : (
          <Table
            size="small"
            rowKey={(r) => `${r.row_idx}-${r.col ?? ''}`}
            pagination={{ pageSize: 15 }}
            dataSource={issues ?? []}
            columns={[
              { title: '行', dataIndex: 'row_key', render: (v, r) => v || `第${r.row_idx + 1}行` },
              { title: '列', dataIndex: 'col', render: (v) => v ?? '-' },
              { title: '值', dataIndex: 'value', render: (v) => String(v ?? '') },
              { title: '说明', dataIndex: 'message' },
            ]}
          />
        )}
      </Drawer>

      <ImportDialog
        open={importOpen}
        code={code}
        params={params}
        onClose={() => setImportOpen(false)}
        onImported={() => {
          msgApi.success('导入完成');
          load();
        }}
      />
    </>
  );
};

export default ReportForm;
