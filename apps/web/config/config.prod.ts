// 生产构建配置
import base from './config';

export default {
  ...(base as Record<string, unknown>),
  publicPath: '/',
};
