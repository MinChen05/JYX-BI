// 开发环境代理：/api → Go 后端
export default {
  dev: {
    '/api/': {
      target: 'http://127.0.0.1:8090',
      changeOrigin: true,
    },
  },
  test: {
    '/api/': {
      target: 'http://127.0.0.1:8090',
      changeOrigin: true,
    },
  },
};
