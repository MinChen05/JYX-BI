import {
  DeleteOutlined,
  DownloadOutlined,
  FileExcelOutlined,
  PlusOutlined,
  PrinterOutlined,
  SaveOutlined,
  SendOutlined,
  UndoOutlined,
} from '@ant-design/icons';
import { Button, Space } from 'antd';
import React from 'react';

interface Props {
  locked: boolean;
  rowOps: { add: boolean; delete: boolean };
  onDraft: () => void;
  onValidate: () => void;
  onAddRow: () => void;
  onDeleteRow: () => void;
  onExport: () => void;
  onImport: () => void;
  onSubmit: () => void;
  onWithdraw: () => void;
  onPrint: () => void;
}

/** 工具条：对齐截图里的 [提交 数据校验 添加记录 删除行 导出 覆盖导入 打印] */
const Toolbar: React.FC<Props> = (p) => {
  return (
    <Space wrap>
      <Button type="primary" icon={<SendOutlined />} onClick={p.onSubmit}>
        提交
      </Button>
      <Button icon={<SaveOutlined />} onClick={p.onDraft} disabled={p.locked}>
        存草稿
      </Button>
      <Button onClick={p.onValidate}>数据校验</Button>
      {p.rowOps.add && (
        <Button icon={<PlusOutlined />} onClick={p.onAddRow} disabled={p.locked}>
          添加记录
        </Button>
      )}
      {p.rowOps.delete && (
        <Button
          danger
          icon={<DeleteOutlined />}
          onClick={p.onDeleteRow}
          disabled={p.locked}
        >
          删除行
        </Button>
      )}
      <Button icon={<FileExcelOutlined />} onClick={p.onExport}>
        导出
      </Button>
      <Button icon={<DownloadOutlined />} onClick={p.onImport} disabled={p.locked}>
        覆盖导入
      </Button>
      {p.locked && (
        <Button icon={<UndoOutlined />} onClick={p.onWithdraw}>
          撤回
        </Button>
      )}
      <Button icon={<PrinterOutlined />} onClick={p.onPrint}>
        打印
      </Button>
    </Space>
  );
};

export default Toolbar;
