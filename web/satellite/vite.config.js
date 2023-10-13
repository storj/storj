// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

import { resolve } from 'path';

import { defineConfig } from 'vite';
import { visualizer } from 'rollup-plugin-visualizer';
import vue from '@vitejs/plugin-vue';
import viteCompression from 'vite-plugin-compression';
import vitePluginRequire from 'vite-plugin-require';
import svgLoader from 'vite-svg-loader';

const productionBrotliExtensions = ['js', 'css', 'ttf', 'woff', 'woff2'];

const plugins = [
    vue(),
    svgLoader({
        svgoConfig: {
            plugins: [{ name: 'removeViewBox', fn: () => {} }],
        },
    }),
    vitePluginRequire(),
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
    // compress chunks only for production mode builds.
    if (mode === 'production') {
        plugins.push(viteCompression({
            algorithm: 'brotliCompress',
            threshold: 1024,
            ext: '.br',
            filter: new RegExp('\\.(' + productionBrotliExtensions.join('|') + ')$'),
        }));
    }

    return {
        base: '/static/dist',
        plugins,
        resolve: {
            alias: {
                '@': resolve(__dirname, './src'),
                '@poc': resolve(__dirname, './vuetify-poc/src'),
                'stream': 'stream-browserify',
                'util': 'util/',
            },
            extensions: ['.js', '.ts', '.svg', '.vue'],
        },
        build: {
            outDir: resolve(__dirname, 'dist'),
            emptyOutDir: true,
            rollupOptions: {
                output: {
                    experimentalMinChunkSize: 50*1024,
                },
            },
            chunkSizeWarningLimit: 3000,
        },
        define: {
            'process.env': {},
            __UI_TYPE__: JSON.stringify('legacy'),
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
