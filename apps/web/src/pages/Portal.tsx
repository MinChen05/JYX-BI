import { FileTextOutlined, FolderFilled, LinkOutlined } from '@ant-design/icons';
import { Empty, Space, Tree, Typography } from 'antd';
import React, { useEffect, useMemo, useState } from 'react';

import { listReports } from '../api/rpt';
import type { ReportInfo } from '@jyx-bi/rpt-types';
import PageShell from '../components/PageShell';
import { defaultParams } from '../utils/reportParams';

type TreeNode = {
  title: string;
  key: string;
  icon?: React.ReactNode;
  selectable?: boolean;
  children?: TreeNode[];
};

/**
 * 报表门户：左侧目录树（板块分组）+ 右侧内嵌填报页，操作对齐帆软报表目录。
 * 外嵌 PageShell（窄图标栏），与深色顶栏组成帆软三栏结构。
 */
const Portal: React.FC = () => {
  const [reports, setReports] = useState<ReportInfo[]>([]);
  const [selected, setSelected] = useState<ReportInfo | null>(null);

  useEffect(() => {
    listReports().then(setReports).catch(() => setReports([]));
  }, []);

  // 按 metadata.group 分组（空 = 未分组），组序 = 首次出现顺序
  const treeData: TreeNode[] = useMemo(() => {
    const groups = new Map<string, ReportInfo[]>();
    reports.forEach((r) => {
      const g = r.group || '未分组';
      if (!groups.has(g)) groups.set(g, []);
      groups.get(g)!.push(r);
    });
    return Array.from(groups.entries()).map(([g, rs]) => ({
      title: g,
      key: `g:${g}`,
      icon: <FolderFilled />,
      selectable: false,
      children: rs.map((r) => ({
        title: r.name,
        key: `r:${r.code}`,
        icon: <FileTextOutlined />,
      })),
    }));
  }, [reports]);

  const onSelect = (keys: React.Key[]) => {
    const k = keys[0];
    if (typeof k !== 'string' || !k.startsWith('r:')) return;
    const r = reports.find((x) => x.code === k.slice(2));
    if (r) setSelected(r);
  };

  const embedUrl = selected
    ? `/embed/${selected.code}?${new URLSearchParams(defaultParams(selected)).toString()}`
    : '';

  return (
    <PageShell>
      <div style={{ display: 'flex', flex: 1, minHeight: 0 }}>
        {/* 左侧目录树 */}
        <div
          style={{
            width: 250,
            flexShrink: 0,
            borderRight: '1px solid #f0f0f0',
            padding: '12px 8px',
            overflow: 'auto',
          }}
        >
          {treeData.length === 0 ? (
            <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="暂无报表" />
          ) : (
            <Tree
              blockNode
              defaultExpandAll
              showIcon
              treeData={treeData}
              onSelect={onSelect}
              selectedKeys={selected ? [`r:${selected.code}`] : []}
            />
          )}
        </div>
        {/* 右侧内容区 */}
        <div style={{ flex: 1, minWidth: 0, display: 'flex', flexDirection: 'column' }}>
          {selected ? (
            <>
              <div
                style={{
                  display: 'flex',
                  justifyContent: 'space-between',
                  alignItems: 'center',
                  padding: '8px 16px',
                  borderBottom: '1px solid #f0f0f0',
                  flexShrink: 0,
                }}
              >
                <Typography.Text strong>{selected.name}</Typography.Text>
                <Space>
                  <a
                    href={`/reports/${selected.code}?${new URLSearchParams(defaultParams(selected)).toString()}`}
                    target="_blank"
                    rel="noreferrer"
                  >
                    <LinkOutlined /> 新标签页打开
                  </a>
                </Space>
              </div>
              <iframe
                key={selected.code}
                src={embedUrl}
                title={selected.name}
                style={{ flex: 1, width: '100%', border: 'none' }}
              />
            </>
          ) : (
            <div
              style={{
                flex: 1,
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'center',
                background: '#fafafa',
              }}
            >
              <Empty description="请从左侧目录选择报表" />
            </div>
          )}
        </div>
      </div>
    </PageShell>
  );
};

export default Portal;
