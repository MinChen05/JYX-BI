import {
  DeleteOutlined,
  PlusOutlined,
  ReloadOutlined,
  SaveOutlined,
  ThunderboltOutlined,
} from '@ant-design/icons';
import {
  App,
  Button,
  Input,
  List,
  Modal,
  Select,
  Space,
  Table,
  Tag,
} from 'antd';
import React, { useCallback, useEffect, useMemo, useState } from 'react';

import {
  deleteTemplate,
  getTemplate,
  listTemplates,
  reloadTemplates,
  saveTemplate,
  sqlPreview,
  type PreviewColumn,
  type SqlPreviewResult,
  type TemplateSummary,
} from '../api/admin';
import PageShell from '../components/PageShell';
import { errMsg } from '../utils/reportParams';

// ---------- YAML 文本抽取（轻量正则，避免前端引入 YAML 库做编辑操作） ----------

type ParamDefLite = { key: string; label: string; type: string };

function metaCode(text: string): string {
  const m = text.match(/^\s*code:\s*([A-Za-z0-9_]+)/m);
  return m ? m[1] : '';
}

function metaName(text: string): string {
  const m = text.match(/^\s*name:\s*(.+)$/m);
  return m ? m[1].trim() : '';
}

function lineIndent(s: string): number {
  const m = s.match(/^[ \t]*/);
  return m ? m[0].replace(/\t/g, '  ').length : 0;
}

/** 抽取 spec.rows.query 的 YAML 块标量（> 或 | 后的缩进块） */
function extractQuery(text: string): string {
  const lines = text.split('\n');
  const start = lines.findIndex((l) => /^\s*query:\s*[>|][+-]?\s*$/.test(l));
  if (start < 0) return '';
  const headIndent = lineIndent(lines[start]);
  const out: string[] = [];
  for (let i = start + 1; i < lines.length; i++) {
    const l = lines[i];
    if (l.trim() === '') {
      out.push('');
      continue;
    }
    if (lineIndent(l) <= headIndent) break;
    out.push(l.trim());
  }
  while (out.length && out[out.length - 1] === '') out.pop();
  return out.join('\n');
}

function extractSource(text: string): string {
  const m = text.match(/^\s*source:\s*(\w+)/m);
  return m ? m[1] : 'mssql';
}

/** 抽取 spec.params（支持 `- { key: x, ... }` 流式与 `- key: x` 块式两种写法） */
function extractParams(text: string): ParamDefLite[] {
  const lines = text.split('\n');
  const start = lines.findIndex((l) => /^\s*params:\s*$/.test(l));
  if (start < 0) return [];
  const headIndent = lineIndent(lines[start]);
  const out: ParamDefLite[] = [];
  let cur: ParamDefLite | null = null;
  for (let i = start + 1; i < lines.length; i++) {
    const l = lines[i];
    if (l.trim() === '') continue;
    if (lineIndent(l) <= headIndent) break;
    if (/^\s*-\s*\{/.test(l)) {
      // 流式：一行一个参数，字段值到 , 或 } 为止
      const get = (k: string) => {
        const m = l.match(new RegExp(`${k}\\s*:\\s*'?\"?([^,\"}]+?)'?\"?\\s*(?=,|}|$)`));
        return m ? m[1].trim() : '';
      };
      const key = get('key');
      if (key) out.push({ key, label: get('label') || key, type: get('type') || 'text' });
      continue;
    }
    const dash = l.match(/^\s*-\s+(.*)$/);
    if (dash) {
      if (cur) out.push(cur);
      cur = { key: '', label: '', type: 'text' };
      parseParamField(cur, dash[1]);
    } else if (cur) {
      parseParamField(cur, l.trim());
    }
  }
  if (cur) out.push(cur);
  return out.filter((p) => p.key);
}

function parseParamField(p: ParamDefLite, frag: string) {
  for (const k of ['key', 'label', 'type'] as const) {
    const m = frag.match(new RegExp(k + ":'?\"?([^,\"']+?)\"?'?[,}]*$"));
    if (m) p[k] = m[1].trim();
  }
}

function sampleValue(type: string): string {
  const now = new Date();
  const ym = `${now.getFullYear()}-${String(now.getMonth() + 1).padStart(2, '0')}`;
  if (type === 'month') return ym;
  if (type === 'date') return `${ym}-01`;
  return 'x';
}

/** 预览列 → 模板 columns 段（与现有模板同构的流式写法） */
function buildColumnsFragment(cols: PreviewColumn[]): string {
  const esc = (s: string) => (/^[A-Za-z0-9_\u4e00-\u9fff ./-]+$/.test(s) ? s : `"${s.replace(/"/g, '\\"')}"`);
  return cols
    .map((c) => `    - { key: ${esc(c.name)}, label: ${esc(c.name)}, type: ${c.type} }`)
    .join('\n');
}

/** 用片段替换文本中 `  columns:` 段（2 空格缩进的 spec 级键，块 = 其后缩进 ≥4 的行） */
function replaceColumnsSection(text: string, fragment: string): string | null {
  const lines = text.split('\n');
  const start = lines.findIndex((l) => /^  columns:\s*$/.test(l));
  if (start < 0) return null;
  let end = start + 1;
  while (end < lines.length && (lines[end].trim() === '' || lineIndent(lines[end]) >= 4)) end++;
  return [...lines.slice(0, start), `  columns:`, fragment, ...lines.slice(end)].join('\n');
}

const NEW_TEMPLATE = (code: string, name: string) => `apiVersion: rpt/v1
kind: Report
metadata:
  code: ${code}
  name: ${name}
  version: 1
  group:
spec:
  params:
    - { key: biz_date, label: 业务日期, type: month, required: true }
  rows:
    source: mssql
    query: >
      SELECT 1 AS col1, N'x' AS col2
  columns:
    - { key: col1, label: 列1, type: int, readonly: true }
    - { key: col2, label: 列2, type: text }
`;

// ---------- 页面 ----------

const Designer: React.FC = () => {
  const { message } = App.useApp();
  const [templates, setTemplates] = useState<TemplateSummary[]>([]);
  const [currentCode, setCurrentCode] = useState('');
  const [yamlText, setYamlText] = useState('');
  const [dirty, setDirty] = useState(false);
  const [saving, setSaving] = useState(false);
  const [saveError, setSaveError] = useState('');
  const [newModal, setNewModal] = useState(false);
  const [newCode, setNewCode] = useState('');
  const [newName, setNewName] = useState('');

  // SQL 预览面板
  const [sqlSource, setSqlSource] = useState('mssql');
  const [sqlText, setSqlText] = useState('');
  const [paramValues, setParamValues] = useState<Record<string, string>>({});
  const [preview, setPreview] = useState<SqlPreviewResult | null>(null);
  const [previewing, setPreviewing] = useState(false);
  const [genModal, setGenModal] = useState(false);
  const [genFrag, setGenFrag] = useState('');

  const paramDefs = useMemo(() => extractParams(yamlText), [yamlText]);

  const refresh = useCallback(async () => {
    try {
      setTemplates(await listTemplates());
    } catch (e) {
      message.error(errMsg(e, '加载模板列表失败'));
    }
  }, [message]);

  useEffect(() => {
    refresh();
  }, [refresh]);

  const loadTemplate = useCallback(
    async (code: string) => {
      try {
        const { raw } = await getTemplate(code);
        setCurrentCode(code);
        setYamlText(raw);
        setDirty(false);
        setSaveError('');
        const q = extractQuery(raw);
        if (q) setSqlText(q);
        setSqlSource(extractSource(raw));
        const defs = extractParams(raw);
        setParamValues(Object.fromEntries(defs.map((d) => [d.key, sampleValue(d.type)])));
      } catch (e) {
        message.error(errMsg(e, '加载模板失败'));
      }
    },
    [message],
  );

  const onEditorChange = (v: string) => {
    setYamlText(v);
    setDirty(true);
    setSaveError('');
  };

  // Tab 键插入 2 空格
  const onEditorKey = (e: React.KeyboardEvent<HTMLTextAreaElement>) => {
    if (e.key !== 'Tab') return;
    e.preventDefault();
    const el = e.currentTarget;
    const { selectionStart: s, selectionEnd: end } = el;
    const next = yamlText.slice(0, s) + '  ' + yamlText.slice(end);
    onEditorChange(next);
    requestAnimationFrame(() => {
      el.selectionStart = el.selectionEnd = s + 2;
    });
  };

  const doSave = async () => {
    const code = metaCode(yamlText);
    if (!code) {
      setSaveError('YAML 中缺少 metadata.code');
      return;
    }
    setSaving(true);
    try {
      await saveTemplate(code, yamlText);
      message.success(`模板 ${code} 已保存并热加载`);
      setCurrentCode(code);
      setDirty(false);
      setSaveError('');
      refresh();
    } catch (e) {
      const msg = errMsg(e, '保存失败');
      setSaveError(msg);
      message.error(msg);
    } finally {
      setSaving(false);
    }
  };

  const doDelete = (code: string) => {
    Modal.confirm({
      title: `删除模板 ${code}？`,
      content: '将删除模板文件并热加载，历史填报实例与数据库数据不受影响。',
      okText: '删除',
      okButtonProps: { danger: true },
      onOk: async () => {
        try {
          await deleteTemplate(code);
          message.success(`模板 ${code} 已删除`);
          if (code === currentCode) {
            setCurrentCode('');
            setYamlText('');
            setDirty(false);
          }
          refresh();
        } catch (e) {
          message.error(errMsg(e, '删除失败'));
        }
      },
    });
  };

  const doReload = async () => {
    try {
      await reloadTemplates();
      message.success('模板目录已重新加载');
      await refresh();
      if (currentCode) loadTemplate(currentCode);
    } catch (e) {
      message.error(errMsg(e, '重载失败'));
    }
  };

  const openNew = () => {
    setNewCode('');
    setNewName('');
    setNewModal(true);
  };

  const doCreate = () => {
    const code = newCode.trim().toLowerCase();
    if (!/^[a-z][a-z0-9_]*$/.test(code)) {
      message.warning('code 只能用小写字母数字下划线，且以字母开头');
      return;
    }
    if (!newName.trim()) {
      message.warning('请填写报表名称');
      return;
    }
    const text = NEW_TEMPLATE(code, newName.trim());
    setCurrentCode(code);
    setYamlText(text);
    setDirty(true);
    setSaveError('');
    setSqlText(extractQuery(text));
    setSqlSource('mssql');
    setParamValues({ biz_date: sampleValue('month') });
    setNewModal(false);
    message.info('已创建新模板，编辑后点「保存」生效');
  };

  const extractFromTpl = () => {
    const q = extractQuery(yamlText);
    if (!q) {
      message.warning('模板里没有 rows.query 块（只支持 > / | 块标量写法）');
      return;
    }
    setSqlText(q);
    setSqlSource(extractSource(yamlText));
    const defs = extractParams(yamlText);
    setParamValues(Object.fromEntries(defs.map((d) => [d.key, sampleValue(d.type)])));
  };

  const doPreview = async () => {
    if (!sqlText.trim()) {
      message.warning('SQL 为空');
      return;
    }
    setPreviewing(true);
    try {
      const res = await sqlPreview(sqlSource, sqlText, paramDefs, paramValues);
      setPreview(res);
    } catch (e) {
      setPreview(null);
      message.error(errMsg(e, 'SQL 预览失败'));
    } finally {
      setPreviewing(false);
    }
  };

  const openGen = () => {
    if (!preview || preview.columns.length === 0) return;
    setGenFrag(buildColumnsFragment(preview.columns));
    setGenModal(true);
  };

  const applyGen = () => {
    const next = replaceColumnsSection(yamlText, genFrag);
    if (next === null) {
      message.warning('模板里没有 `columns:` 段，未替换——请手动粘贴');
      return;
    }
    onEditorChange(next);
    setGenModal(false);
    message.success('columns 段已替换为预览列');
  };

  const tableColumns = useMemo(
    () =>
      preview?.columns.map((c, i) => ({
        title: `${c.name} [${c.db_type} → ${c.type}]`,
        dataIndex: i,
        key: i,
        ellipsis: true,
        render: (v: unknown) => (v === null || v === undefined ? <Tag>NULL</Tag> : String(v)),
      })) ?? [],
    [preview],
  );

  return (
    <PageShell>
      <div style={{ display: 'flex', height: '100%' }}>
        {/* 左：模板列表 */}
        <div
          style={{
            width: 240,
            flexShrink: 0,
            borderRight: '1px solid #f0f0f0',
            display: 'flex',
            flexDirection: 'column',
          }}
        >
          <div
            style={{
              padding: '10px 12px',
              borderBottom: '1px solid #f0f0f0',
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'space-between',
            }}
          >
            <b>模板</b>
            <Space size={4}>
              <Button size="small" icon={<ReloadOutlined />} title="重载目录" onClick={doReload} />
              <Button size="small" type="primary" icon={<PlusOutlined />} onClick={openNew} />
            </Space>
          </div>
          <div style={{ flex: 1, overflow: 'auto' }}>
            <List
              size="small"
              dataSource={templates}
              renderItem={(t) => (
                <List.Item
                  style={{
                    padding: '8px 12px',
                    cursor: 'pointer',
                    background: t.code === currentCode ? '#e6f4ff' : undefined,
                    display: 'block',
                  }}
                  onClick={() => loadTemplate(t.code)}
                  actions={[
                    <DeleteOutlined
                      key="del"
                      style={{ color: '#ff4d4f' }}
                      onClick={(e) => {
                        e.stopPropagation();
                        doDelete(t.code);
                      }}
                    />,
                  ]}
                >
                  <List.Item.Meta
                    title={
                      <span>
                        {t.name}
                        {t.group && (
                          <Tag style={{ marginLeft: 6 }} color="blue">
                            {t.group}
                          </Tag>
                        )}
                      </span>
                    }
                    description={`${t.code} · v${t.version}${t.has_submit ? ' · 填报' : ' · 展示'}`}
                  />
                </List.Item>
              )}
            />
          </div>
        </div>

        {/* 中：YAML 编辑器 */}
        <div style={{ flex: 1.1, minWidth: 0, display: 'flex', flexDirection: 'column' }}>
          <div
            style={{
              padding: '8px 12px',
              borderBottom: '1px solid #f0f0f0',
              display: 'flex',
              alignItems: 'center',
              gap: 8,
            }}
          >
            <b>{currentCode || '未选择模板'}</b>
            {metaName(yamlText) && <Tag>{metaName(yamlText)}</Tag>}
            {dirty && <Tag color="orange">未保存</Tag>}
            <span style={{ flex: 1 }} />
            <Button
              type="primary"
              icon={<SaveOutlined />}
              loading={saving}
              disabled={!dirty && !!currentCode}
              onClick={doSave}
            >
              保存
            </Button>
          </div>
          {saveError && (
            <div
              style={{
                padding: '6px 12px',
                background: '#fff2f0',
                borderBottom: '1px solid #ffccc7',
                color: '#cf1322',
                fontSize: 12.5,
              }}
            >
              {saveError}
            </div>
          )}
          <textarea
            value={yamlText}
            onChange={(e) => onEditorChange(e.target.value)}
            onKeyDown={onEditorKey}
            spellCheck={false}
            placeholder={'从左侧选择模板，或点「新建」开始'}
            style={{
              flex: 1,
              border: 'none',
              outline: 'none',
              resize: 'none',
              padding: 12,
              fontFamily: 'ui-monospace, SFMono-Regular, Menlo, Consolas, monospace',
              fontSize: 12.5,
              lineHeight: 1.65,
              background: '#fafafa',
            }}
          />
        </div>

        {/* 右：SQL 预览 */}
        <div
          style={{
            flex: 1,
            minWidth: 0,
            borderLeft: '1px solid #f0f0f0',
            display: 'flex',
            flexDirection: 'column',
          }}
        >
          <div
            style={{
              padding: '8px 12px',
              borderBottom: '1px solid #f0f0f0',
              display: 'flex',
              alignItems: 'center',
              gap: 8,
            }}
          >
            <b>SQL 预览</b>
            <Select
              size="small"
              value={sqlSource}
              onChange={setSqlSource}
              style={{ width: 110 }}
              options={[
                { value: 'mssql', label: 'MSSQL (ERP)' },
                { value: 'doris', label: 'Doris (数仓)' },
              ]}
            />
            <Button size="small" onClick={extractFromTpl}>
              从模板提取
            </Button>
            <span style={{ flex: 1 }} />
            <Button size="small" type="primary" icon={<ThunderboltOutlined />} loading={previewing} onClick={doPreview}>
              运行
            </Button>
            <Button size="small" onClick={openGen} disabled={!preview || preview.columns.length === 0}>
              生成列
            </Button>
          </div>
          <div style={{ padding: '8px 12px', borderBottom: '1px solid #f0f0f0' }}>
            <textarea
              value={sqlText}
              onChange={(e) => setSqlText(e.target.value)}
              spellCheck={false}
              placeholder={'SELECT ... （参数用 {key} 占位，仅允许单条 SELECT/WITH）'}
              style={{
                width: '100%',
                height: 150,
                border: '1px solid #d9d9d9',
                borderRadius: 6,
                padding: 8,
                fontFamily: 'ui-monospace, SFMono-Regular, Menlo, Consolas, monospace',
                fontSize: 12.5,
                lineHeight: 1.6,
                resize: 'vertical',
                outline: 'none',
              }}
            />
            {paramDefs.length > 0 && (
              <div style={{ display: 'flex', gap: 8, marginTop: 8, flexWrap: 'wrap' }}>
                {paramDefs.map((p) => (
                  <div key={p.key} style={{ display: 'flex', alignItems: 'center', gap: 4 }}>
                    <span style={{ fontSize: 12, color: 'rgba(0,0,0,0.55)' }}>{p.label}</span>
                    <Input
                      size="small"
                      style={{ width: 110 }}
                      value={paramValues[p.key] ?? ''}
                      placeholder={sampleValue(p.type)}
                      onChange={(e) => setParamValues({ ...paramValues, [p.key]: e.target.value })}
                    />
                  </div>
                ))}
              </div>
            )}
          </div>
          <div style={{ flex: 1, overflow: 'auto', padding: 12 }}>
            {preview ? (
              <Table
                size="small"
                bordered
                columns={tableColumns}
                dataSource={(preview.rows as unknown[][]).map((r, i) => ({
                  _id: i,
                  ...Object.fromEntries(r.map((v, j) => [j, v])),
                }))}
                rowKey="_id"
                pagination={false}
                scroll={{ x: 'max-content' }}
              />
            ) : (
              <div style={{ color: 'rgba(0,0,0,0.35)', fontSize: 13, padding: 24 }}>
                点「运行」查看查询结果（限 100 行）；「生成列」把结果列转成模板 columns 定义
              </div>
            )}
          </div>
        </div>
      </div>

      {/* 新建模板 */}
      <Modal
        title="新建模板"
        open={newModal}
        onOk={doCreate}
        okText="创建"
        cancelText="取消"
        onCancel={() => setNewModal(false)}
      >
        <div style={{ display: 'flex', flexDirection: 'column', gap: 12, padding: '8px 0' }}>
          <div>
            <div style={{ marginBottom: 4 }}>模板 code（小写字母数字下划线）</div>
            <Input
              value={newCode}
              onChange={(e) => setNewCode(e.target.value)}
              placeholder="如 sales_recon_monthly"
            />
          </div>
          <div>
            <div style={{ marginBottom: 4 }}>报表名称</div>
            <Input
              value={newName}
              onChange={(e) => setNewName(e.target.value)}
              placeholder="如 销售对账月报"
            />
          </div>
          <div style={{ fontSize: 12, color: 'rgba(0,0,0,0.45)' }}>
            创建后可在中间编辑 YAML、右侧调试 SQL；点「保存」才会写入 templates/ 目录并热加载。
          </div>
        </div>
      </Modal>

      {/* 生成列 */}
      <Modal
        title="从预览结果生成 columns 段"
        open={genModal}
        onCancel={() => setGenModal(false)}
        width={640}
        footer={[
          <Button key="copy" onClick={() => navigator.clipboard.writeText(genFrag).then(() => message.success('已复制'))}>
            复制
          </Button>,
          <Button key="cancel" onClick={() => setGenModal(false)}>
            取消
          </Button>,
          <Button key="apply" type="primary" onClick={applyGen}>
            替换模板 columns 段
          </Button>,
        ]}
      >
        <div style={{ fontSize: 12, color: 'rgba(0,0,0,0.45)', marginBottom: 8 }}>
          按预览列名与推断类型（text/int/money/date）生成。
          <span style={{ color: '#cf1322' }}>
            替换会整段覆盖现有 columns（丢失 readonly / dynamic / width / formula 等属性），请确认后再操作。
          </span>
        </div>
        <pre
          style={{
            background: '#fafafa',
            border: '1px solid #f0f0f0',
            borderRadius: 6,
            padding: 12,
            fontSize: 12.5,
            lineHeight: 1.7,
            overflow: 'auto',
            maxHeight: 320,
          }}
        >
          {`  columns:\n${genFrag}`}
        </pre>
      </Modal>
    </PageShell>
  );
};

export default Designer;
