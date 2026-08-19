import { CheckCircleOutlined, WarningOutlined } from '@ant-design/icons';
import {
  Alert,
  Button,
  Descriptions,
  Modal,
  Space,
  Table,
  Upload,
} from 'antd';
import React, { useState } from 'react';

import { importFile } from '../api/rpt';
import type { ImportReport } from '@kingdee-rpt/rpt-types';

interface Props {
  open: boolean;
  code: string;
  params: Record<string, string>;
  onClose: () => void;
  onImported: () => void;
}

/** 覆盖导入对话框：上传 → 校验报告 + 变更 diff → 确认落地 */
const ImportDialog: React.FC<Props> = ({ open, code, params, onClose, onImported }) => {
  const [report, setReport] = useState<ImportReport | null>(null);
  const [loading, setLoading] = useState(false);

  const doImport = async (file: File) => {
    setLoading(true);
    try {
      const r = await importFile(code, params, file);
      setReport(r);
    } finally {
      setLoading(false);
    }
    return false; // 阻止 antd 默认上传
  };

  const confirm = async () => {
    if (!report) return;
    const { confirmImport } = await import('../api/rpt');
    await confirmImport(code, params, report.job_id);
    onImported();
    onClose();
  };

  return (
    <Modal
      title="覆盖导入"
      open={open}
      width={860}
      onCancel={onClose}
      footer={
        report &&
        report.errors.length === 0 ? (
          <Space>
            <Button onClick={onClose}>取消</Button>
            <Button type="primary" onClick={confirm}>
              确认导入（{report.changed} 格变更）
            </Button>
          </Space>
        ) : null
      }
    >
      {!report && (
        <Upload.Dragger
          accept=".xlsx"
          showUploadList={false}
          beforeUpload={doImport}
          disabled={loading}
        >
          <p className="ant-upload-drag-icon">
            <DownloadOutlinedLike />
          </p>
          <p className="ant-upload-text">
            {loading ? '解析校验中…' : '选择导出的 xlsx 文件（须为本报表同参数导出）'}
          </p>
        </Upload.Dragger>
      )}
      {report && (
        <Space direction="vertical" style={{ width: '100%' }} size="middle">
          {report.errors.length > 0 ? (
            <Alert
              type="error"
              icon={<WarningOutlined />}
              message={`校验未通过（${report.errors.length} 项），不能确认导入`}
              description={
                <Table
                  size="small"
                  pagination={false}
                  dataSource={report.errors.slice(0, 20)}
                  columns={[
                    { title: '行', dataIndex: 'row_key' },
                    { title: '列', dataIndex: 'col', render: (v) => v ?? '-' },
                    { title: '值', dataIndex: 'value', render: (v) => String(v ?? '') },
                    { title: '说明', dataIndex: 'message' },
                  ]}
                />
              }
            />
          ) : (
            <Alert
              type="success"
              icon={<CheckCircleOutlined />}
              message={`校验通过，共 ${report.changed} 格变更`}
            />
          )}
          {report.unmatched.length > 0 && (
            <Alert
              type="warning"
              message={`以下 ${report.unmatched.length} 行未匹配到当前行集（将被忽略）：${report.unmatched.join('、')}`}
            />
          )}
          <Descriptions size="small" column={3} bordered>
            <Descriptions.Item label="作业 ID">{report.job_id}</Descriptions.Item>
            <Descriptions.Item label="状态">{report.status}</Descriptions.Item>
            <Descriptions.Item label="变更格数">{report.changed}</Descriptions.Item>
          </Descriptions>
          {report.cells.length > 0 && (
            <Table
              size="small"
              rowKey={(r) => `${r.row_key}-${r.col}`}
              pagination={{ pageSize: 8 }}
              dataSource={report.cells}
              columns={[
                { title: '行', dataIndex: 'row_key' },
                { title: '列', dataIndex: 'col' },
                { title: '原值', dataIndex: 'old', render: (v) => String(v ?? '∅') },
                { title: '新值', dataIndex: 'new', render: (v) => String(v ?? '∅') },
              ]}
            />
          )}
        </Space>
      )}
    </Modal>
  );
};

// 简单的占位图标（避免再引一个 icon）
const DownloadOutlinedLike: React.FC = () => (
  <span style={{ fontSize: 30 }}>⬇</span>
);

export default ImportDialog;
