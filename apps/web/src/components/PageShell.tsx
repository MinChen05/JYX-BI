import { AppstoreOutlined, BuildOutlined, TableOutlined } from '@ant-design/icons';
import React from 'react';

type RailItem = { key: string; label: string; icon: React.ReactNode; path: string };

const RAIL: RailItem[] = [
  { key: 'catalog', label: '目录', icon: <AppstoreOutlined />, path: '/portal' },
  { key: 'list', label: '报表清单', icon: <TableOutlined />, path: '/reports' },
  { key: 'designer', label: '报表设计', icon: <BuildOutlined />, path: '/designer' },
];

/**
 * 帆软式页面壳：左侧窄图标栏（目录/报表清单/报表设计）+ 右侧内容，
 * 与深色顶栏（navTheme: dark）组成帆软工作台的三栏结构。
 */
const PageShell: React.FC<{ children: React.ReactNode }> = ({ children }) => {
  const current = window.location.pathname;
  return (
    <div style={{ display: 'flex', height: 'calc(100vh - 56px)', background: '#fff' }}>
      <div
        style={{
          width: 76,
          flexShrink: 0,
          borderRight: '1px solid #f0f0f0',
          paddingTop: 16,
          display: 'flex',
          flexDirection: 'column',
          alignItems: 'center',
          gap: 8,
        }}
      >
        {RAIL.map((it) => {
          const active = current.startsWith(it.path);
          return (
            <a
              key={it.key}
              href={it.path}
              style={{
                width: 58,
                display: 'flex',
                flexDirection: 'column',
                alignItems: 'center',
                gap: 4,
                padding: '8px 0',
                borderRadius: 6,
                color: active ? '#1677ff' : 'rgba(0,0,0,0.55)',
                background: active ? '#e6f4ff' : 'transparent',
                fontSize: 12,
                textDecoration: 'none',
              }}
            >
              <span style={{ fontSize: 18 }}>{it.icon}</span>
              {it.label}
            </a>
          );
        })}
      </div>
      <div style={{ flex: 1, minWidth: 0, display: 'flex', flexDirection: 'column' }}>{children}</div>
    </div>
  );
};

export default PageShell;
