import { defineConfig } from 'vite';
import vue from '@vitejs/plugin-vue2';
import legacy from '@vitejs/plugin-legacy';
import { nodePolyfills } from 'vite-plugin-node-polyfills';
import { fileURLToPath, URL } from 'node:url';

export default defineConfig({
  // Relative base so the built SPA works under any host prefix (e.g. /sqlui/).
  base: './',
  plugins: [
    vue(),
    // IE11 (政企 environments): emit an ES5 nomodule bundle with core-js
    // polyfills. Modern browsers load the normal module bundle.
    legacy({
      targets: ['ie >= 11'],
      additionalLegacyPolyfills: ['regenerator-runtime/runtime'],
      renderLegacyChunks: true,
    }),
    // Only the browser globals; the SQL app has no Node crypto/socket code.
    nodePolyfills({
      exclude: ['crypto'],
      protocolImports: false,
      globals: {
        Buffer: true,
        process: true,
        global: true,
        globalThis: true,
      },
    }),
  ],
  resolve: {
    extensions: ['.mjs', '.js', '.mts', '.ts', '.jsx', '.tsx', '.json', '.vue', '.scss'],
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url)),
    },
  },
  define: {
    'process.env': {},
    global: 'globalThis',
  },
  optimizeDeps: {
    include: ['element-ui', 'vxe-table', 'codemirror', 'axios'],
  },
  build: {
    outDir: 'dist',
    chunkSizeWarningLimit: 4000,
    assetsInlineLimit: 0,
  },
});
