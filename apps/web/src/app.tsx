import type { RunTimeLayoutConfig } from '@umijs/max';
import { App as AntApp, ConfigProvider } from 'antd';
import React from 'react';
import zhCN from 'antd/locale/zh_CN';

import defaultSettings from '../config/defaultSettings';

/**
 * 初始化状态（v1 内网部署，无登录；SSO 留 v2）
 */
export async function getInitialState() {
  return { name: 'admin' };
}

/** 运行时布局配置 */
export const layout: RunTimeLayoutConfig = ({ initialState }) => {
  return {
    ...defaultSettings,
    rightContentRender: false,
    menuItemRender: (item: any, dom: React.ReactNode) => <a>{dom}</a>,
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
