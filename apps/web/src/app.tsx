import type { RunTimeLayoutConfig } from '@umijs/max';
import { App as AntApp, ConfigProvider } from 'antd';
import React from 'react';
import zhCN from 'antd/locale/zh_CN';

/**
 * 初始化状态（v1 内网部署，无登录；SSO 留 v2）
 */
export async function getInitialState() {
  return { name: 'admin' };
}

/** 运行时布局配置（外观 navTheme 由 config.ts 的 layout 配置生成，
 *  layout 模式必须在这里提供——插件模板不会透传 config 里的 layout） */
export const layout: RunTimeLayoutConfig = ({ initialState }) => {
  return {
    layout: 'top',
    title: '金蝶报表平台',
    // 只把顶栏染成深色（帆软式深色顶栏 + 浅色内容区）；
    // 不能用 navTheme: realDark——那会给整个 ProConfigProvider 开暗色算法
    token: {
      header: {
        colorBgHeader: '#001529',
        colorBgScrollHeader: '#001529',
        colorHeaderTitle: '#fff',
        colorTextMenu: 'rgba(255,255,255,0.75)',
        colorTextMenuSecondary: 'rgba(255,255,255,0.65)',
        colorTextMenuActive: '#fff',
        colorTextMenuSelected: '#fff',
        colorBgMenuItemSelected: '#1677ff',
        colorBgMenuItemHover: 'rgba(255,255,255,0.08)',
        colorBgMenuElevated: '#001529',
        colorBgRightActionsItemHover: 'rgba(255,255,255,0.08)',
        colorTextRightActionsItem: 'rgba(255,255,255,0.85)',
        heightLayoutHeader: 56,
      },
    },
    // 顶栏右上角用户角（对齐帆软右上角头像区）
    avatarProps: {
      size: 'small',
      title: (initialState as any)?.name ?? 'admin',
    },
    footerRender: false,
    waterMarkProps: {
      content: (initialState as any)?.name,
    },
  };
};

/** request 错误处理：统一弹错 */
export const request = {
  timeout: 60000,
  requestInterceptors: [],
  responseInterceptors: [
    (response: Response) => {
      return response;
    },
  ],
  errorConfig: {
    errorHandler: (error: any) => {
      if (error?.response?.status === 401) {
        return;
      }
      throw error;
    },
  },
};

export function rootContainer(container: React.ReactNode) {
  return (
    <ConfigProvider locale={zhCN}>
      <AntApp>{container}</AntApp>
    </ConfigProvider>
  );
}
