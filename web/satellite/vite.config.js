// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

import { resolve } from 'path';

import vue from '@vitejs/plugin-vue';
import vuetify, { transformAssetUrls } from 'vite-plugin-vuetify';
import { defineConfig } from 'vite';
import { visualizer } from 'rollup-plugin-visualizer';
import viteCompression from 'vite-plugin-compression2';
import { checker } from 'vite-plugin-checker';

import papaParseWorker from './vitePlugins/papaParseWorker';

const productionBrotliExtensions = ['js', 'css', 'ttf', 'woff', 'woff2'];

const plugins = [
    vue({
        template: { transformAssetUrls },
    }),
    vuetify({
        autoImport: true,
        styles: {
            configFile: 'src/styles/settings.scss',
        },
    }),
    checker({ typescript: true, vueTsc: true }),
];

if (process.env['STORJ_DEBUG_BUNDLE_SIZE']) {
    plugins.push(visualizer({
        template: 'treemap', // or sunburst
        open: true,
        brotliSize: true,
        filename: 'analyse.html', // will be saved in project's root
    }));
}

export default defineConfig(({ mode }) => {
    const isProd = mode === 'production';

    if (isProd) {
        plugins.push(papaParseWorker());
        plugins.push(viteCompression({
            algorithms: ['brotliCompress'],
            threshold: 1024,
            ext: '.br',
            filter: new RegExp('\\.(' + productionBrotliExtensions.join('|') + ')$'),
        }));
    } else {
        // Provide a stub for the papa parse worker in DEV mode.
        plugins.push({
            name: 'papa-parse-worker-dev-stub',
            resolveId(id) {
                if (id === 'virtual:papa-parse-worker') {
                    return id;
                }
            },
            load(id) {
                if (id === 'virtual:papa-parse-worker') {
                    return 'export default null;';
                }
            },
        });
        process.env['NODE_ENV'] = 'development';
    }

    return {
        base: isProd ? '/static/dist' : '/',
        plugins,
        define: {
            'process.env': {},
            global: 'globalThis',
        },
        server: {
            port: 3000,
            host: true,
            proxy: {
                '/api': {
                    target: 'http://localhost:10000',
                    changeOrigin: true,
                    secure: false,
                },
                '/static/static': {
                    target: 'http://localhost:10000',
                    changeOrigin: true,
                    secure: false,
                },
            },
        },
        publicDir: isProd ? '' : 'static',
        resolve: {
            alias: {
                '@': resolve(__dirname, './src'),
                'stream': 'stream-browserify', // Passphrase mnemonic generation will not work without this
                'util': 'util/',
            },
            extensions: [
                '.js',
                '.json',
                '.jsx',
                '.mjs',
                '.ts',
                '.tsx',
                '.vue',
            ],
        },
        build: {
            outDir: resolve(__dirname, 'dist'),
            emptyOutDir: true,
            reportCompressedSize: isProd,
            rollupOptions: {
                output: {
                    experimentalMinChunkSize: 150*1024, // 150KB
                    manualChunks: (id) => {
                        if (id.includes('node_modules')) {
                            if (id.includes('vuetify')) return 'vendor-ui';
                            if (id.includes('vue') || id.includes('pinia') || id.includes('vue-router')) return 'vendor-vue';
                            if (id.includes('lucide')) return 'vendor-icons';
                            if (id.includes('chart.js')) return 'vendor-charts';
                            if (id.includes('papaparse')) return 'vendor-utils';
                            // Keep AWS SDK in vendor-misc to avoid circular deps.
                            return 'vendor-misc';
                        }

                        if (id.includes('/dialogs/') || id.includes('Dialog.vue')) {
                            return 'feature-dialogs';
                        }
                        if (id.includes('/components/common/')) {
                            return 'components-common';
                        }
                        if (id.includes('/components/') && (id.includes('Icon') || id.includes('icon'))) {
                            return 'components-icons';
                        }
                    },
                    chunkFileNames: (chunkInfo) => {
                        if (chunkInfo.name && chunkInfo.name.startsWith('vendor-')) {
                            return 'vendors/[name]-[hash].js';
                        }
                        if (chunkInfo.name && chunkInfo.name.startsWith('feature-')) {
                            return 'features/[name]-[hash].js';
                        }
                        if (chunkInfo.name && chunkInfo.name.startsWith('components-')) {
                            return 'components/[name]-[hash].js';
                        }
                        return 'chunks/[name]-[hash].js';
                    },
                },
            },
            chunkSizeWarningLimit: 3000,
        },
        test: {
            globals: true,
            environment: 'jsdom',
            setupFiles: [
                './vitest.setup.ts',
            ],
            exclude: [
                '**/node_modules/**',
                '**/dist/**',
            ],
        },
    };
});
