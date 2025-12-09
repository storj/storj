// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

import { resolve } from 'path';

import { defineConfig } from 'vite';
import vue from '@vitejs/plugin-vue';
import vuetify, { transformAssetUrls } from 'vite-plugin-vuetify';
import svgLoader from 'vite-svg-loader';

const plugins = [
    vue({
        template: { transformAssetUrls },
    }),
    svgLoader(),
    vuetify({
        autoImport: true,
    }),
];

export default defineConfig(({ mode }) => {
    const isProd = mode === 'production';
    if (!isProd) {
        process.env['NODE_ENV'] = 'development';
    }

    return {
        base: isProd ? '/static/' : '/',
        plugins,
        define: {
            'process.env': {},
            global: 'globalThis',
        },
        server: {
            port: 3002,
            host: true,
            proxy: {
                '/api': {
                    target: 'http://localhost:40000',
                    changeOrigin: true,
                    secure: false,
                },
                '/static/static': {
                    target: 'http://localhost:40000',
                    changeOrigin: true,
                    secure: false,
                },
            },
        },
        resolve: {
            alias: {
                '@': resolve(__dirname, './src'),
            },
            extensions: ['.js', '.ts', '.svg', '.vue'],
        },
        build: {
            outDir: resolve(__dirname, 'dist'),
            emptyOutDir: true,
            reportCompressedSize: false,
            rollupOptions: {
                output: {
                    experimentalMinChunkSize: 150*1024, // 150KB
                    manualChunks: (id) => {
                        if (id.includes('node_modules')) {
                            if (id.includes('vuetify')) return 'vendor-ui';
                            if (id.includes('vue') || id.includes('pinia') || id.includes('vue-router')) return 'vendor-vue';
                            if (id.includes('chart.js')) return 'vendor-charts';
                            return 'vendor-misc';
                        }
                    },
                    chunkFileNames: (chunkInfo) => {
                        if (chunkInfo.name && chunkInfo.name.startsWith('vendor-')) {
                            return 'vendors/[name]-[hash].js';
                        }
                        return 'chunks/[name]-[hash].js';
                    },
                },
            },
            chunkSizeWarningLimit: 3000,
        },
        css: {
            preprocessorOptions: {
                scss: {
                    silenceDeprecations: ['import'],
                },
            },
        },
    };
});
