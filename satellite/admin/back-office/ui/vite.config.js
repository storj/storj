// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

// Plugins
import { resolve } from 'path';
import { existsSync, writeFileSync } from 'fs';

import vue from '@vitejs/plugin-vue';
import vuetify, { transformAssetUrls } from 'vite-plugin-vuetify';
import { defineConfig } from 'vite';
import { checker } from 'vite-plugin-checker';
import viteCompression from 'vite-plugin-compression2';

const productionBrotliExtensions = ['js', 'css', 'ttf', 'woff', 'woff2'];

const plugins = [
    {
        name: 'add-git-keep',
        closeBundle() {
            const file = resolve(__dirname, 'build/.keep');
            if (!existsSync(file)) {
                writeFileSync(file, '');
            }
        },
    },
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

const backOfficeBaseURL = '/back-office';
// https://vitejs.dev/config/
export default defineConfig(({ mode }) => {
    switch (mode) {
    case 'satellite-dev': // serving through the satellite or oauth2 proxy
    case 'development': // serving through `vite dev` server
        process.env['NODE_ENV'] = 'development';
        break;
    default:
        process.env['NODE_ENV'] = 'production';
    }
    const isProd = mode === 'production';
    const isDev = mode === 'development';

    if (isProd) {
        plugins.push(viteCompression({
            algorithm: 'brotliCompress',
            threshold: 1024,
            ext: '.br',
            filter: new RegExp('\\.(' + productionBrotliExtensions.join('|') + ')$'),
        }));
    }

    return {
        base: isDev ? '/' : backOfficeBaseURL + '/static/build',
        plugins: plugins,
        define: {
            global: 'globalThis',
            'process.env': {
                'BASE_URL': isDev ? '' : backOfficeBaseURL,
            },
        },
        server: {
            port: 3000,
            host: true,
            proxy: {
                '/back-office': {
                    target: 'http://localhost:9080',
                    changeOrigin: true,
                    secure: false,
                },
            },
        },
        publicDir: !isDev ? '' : 'static',
        resolve: {
            alias: {
                '@': resolve(__dirname, './src'),
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
            outDir: resolve(__dirname, 'build'),
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
    };
});
