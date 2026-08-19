import { DatePicker, Space, Typography } from 'antd';
import dayjs from 'dayjs';
import React from 'react';

import type { ParamDef } from '@jyx-bi/rpt-types';

interface Props {
  params: ParamDef[];
  values: Record<string, string>;
  onChange: (next: Record<string, string>) => void;
}

/** 参数栏：月份/日期/文本参数，变更后由父组件重新加载网格 */
const ParamBar: React.FC<Props> = ({ params, values, onChange }) => {
  if (params.length === 0) return null;
  return (
    <Space size="middle" wrap>
      {params.map((p) => {
        const v = values[p.key];
        return (
          <Space key={p.key} size="small">
            <Typography.Text type="secondary">{p.label}</Typography.Text>
            {p.type === 'month' && (
              <DatePicker
                picker="month"
                allowClear={false}
                value={v ? dayjs(v, 'YYYY-MM') : undefined}
                onChange={(_, s) => s && onChange({ ...values, [p.key]: s })}
              />
            )}
            {p.type === 'date' && (
              <DatePicker
                allowClear={false}
                value={v ? dayjs(v, 'YYYY-MM-DD') : undefined}
                onChange={(_, s) => s && onChange({ ...values, [p.key]: s })}
              />
            )}
            {p.type === 'text' && (
              <input
                value={v ?? ''}
                onChange={(e) => onChange({ ...values, [p.key]: e.target.value })}
                style={{ width: 120 }}
              />
            )}
          </Space>
        );
      })}
    </Space>
  );
};

export default ParamBar;
