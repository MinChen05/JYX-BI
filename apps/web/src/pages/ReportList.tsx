import type { ProColumns } from '@ant-design/pro-components';
import { ProTable } from '@ant-design/pro-components';
import { Tag } from 'antd';
import React, { useRef, useState } from 'react';

import { listReports } from '../api/rpt';
import type { ReportInfo } from '@jyx-bi/rpt-types';
import { STATUS_TEXT } from '@jyx-bi/rpt-types';
import { defaultParams, reportUrl } from '../utils/reportParams';
import PageShell from '../components/PageShell';

const ReportList: React.FC = () => {
  const [rows, setRows] = useState<ReportInfo[]>([]);

  const columns: ProColumns<ReportInfo>[] = [
    { title: '报表名称', dataIndex: 'name' },
    { title: '模板编码', dataIndex: 'code', search: false },
    { title: '版本', dataIndex: 'version', width: 70, search: false },
    {
      title: '状态',
      dataIndex: 'instances',
      search: false,
      width: 220,
      render: (_, r) =>
        !r.instances || r.instances.length === 0 ? (
          <Tag>未填报</Tag>
        ) : (
          r.instances.map((i) => (
            <Tag
              key={JSON.stringify(i.params)}
              color={i.status === 1 ? 'green' : 'orange'}
            >
              {Object.entries(i.params)
                .map(([k, v]) => `${k}=${v}`)
                .join(' ')}{' '}
              {STATUS_TEXT[i.status]}
            </Tag>
          ))
        ),
    },
    {
      title: '操作',
      search: false,
      width: 120,
      render: (_, r) => (
        <a
          onClick={() => (window.location.href = reportUrl(r.code, defaultParams(r)))}
        >
          打开填报
        </a>
      ),
    },
  ];

  const actionRef = useRef<any>();

  return (
    <PageShell>
      <div style={{ flex: 1, overflow: 'auto', padding: 16 }}>
        <ProTable<ReportInfo>
        rowKey="code"
        headerTitle="可填报报表"
        columns={columns}
        search={false}
        options={false}
        pagination={false}
        actionRef={actionRef}
        request={async () => {
          const data = (await listReports()) as ReportInfo[];
          setRows(data);
          return { data, success: true };
        }}
        onRow={(r) => ({
          onClick: () => (window.location.href = reportUrl(r.code, defaultParams(r))),
          style: { cursor: 'pointer' },
        })}
        />
      </div>
    </PageShell>
  );
};

export default ReportList;
