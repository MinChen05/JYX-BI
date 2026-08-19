// https://umijs.org/config/

import { join } from 'node:path';
import { defineConfig } from '@umijs/max';
import defaultSettings from './defaultSettings';
import proxy from './proxy';
import routes from './routes';

const { UMI_ENV = 'dev' } = process.env;

export default defineConfig({
  alias: {
    '@kingdee-rpt/rpt-types': join(__dirname, '../../../packages/rpt-types/src'),
  },
  hash: true,
  publicPath: '/',
  routes,
  ignoreMomentLocale: true,
  proxy: proxy[UMI_ENV as keyof typeof proxy],
  antd: {},
  model: {},
  request: {},
  layout: {
    ...defaultSettings,
    title: '金蝶报表平台',
  },
  npmClient: 'npm',
  esbuildMinifyIIFE: true,
});
