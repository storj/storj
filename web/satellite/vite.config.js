// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

import { resolve } from 'path';

import vue from '@vitejs/plugin-vue';
import vuetify, { transformAssetUrls } from 'vite-plugin-vuetify';
import { defineConfig } from 'vite';
import { visualizer } from 'rollup-plugin-visualizer';
import viteCompression from 'vite-plugin-compression';

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
    papaParseWorker(),
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

    // compress chunks only for production mode builds.
    if (isProd) {
        plugins.push(viteCompression({
            algorithm: 'brotliCompress',
            threshold: 1024,
            ext: '.br',
            filter: new RegExp('\\.(' + productionBrotliExtensions.join('|') + ')$'),
        }));
    } else {
        process.env['NODE_ENV'] = 'development';
    }

    return {
        base: '/static/dist',
        plugins,
        define: {
            'process.env': {},
        },
        resolve: {
            alias: {
                '@': resolve(__dirname, './src'),
                'stream': 'stream-browserify', // Passphrase mnemonic generation will not work without this
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
                    experimentalMinChunkSize: 50*1024,
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
                '**/tests/unit/ignore/**',
            ],
        },
    };
});
