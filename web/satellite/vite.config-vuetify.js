// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

import { resolve } from 'path';

import { defineConfig } from 'vite';
import { visualizer } from 'rollup-plugin-visualizer';
import vue from '@vitejs/plugin-vue';
import viteCompression from 'vite-plugin-compression';
import vitePluginRequire from 'vite-plugin-require';
import svgLoader from 'vite-svg-loader';
import vuetify, { transformAssetUrls } from 'vite-plugin-vuetify';

const productionBrotliExtensions = ['js', 'css', 'ttf', 'woff', 'woff2'];

const plugins = [
    vue({
        template: { transformAssetUrls }
    }),
    svgLoader({
        svgoConfig: {
            plugins: [{ name: 'removeViewBox', fn: () => {} }],
        },
    }),
    vitePluginRequire(),
    vuetify({
        autoImport: true,
        styles: {
            configFile: 'vuetify-poc/src/styles/settings.scss',
        },
    }),
];

if (process.env.NODE_ENV === 'production') {
    plugins.push(viteCompression({
        algorithm: 'brotliCompress',
        threshold: 1024,
        ext: '.br',
        filter: new RegExp('\\.(' + productionBrotliExtensions.join('|') + ')$'),
    }));
}

if (process.env['STORJ_DEBUG_BUNDLE_SIZE']) {
    plugins.push(visualizer({
        open: true,
        brotliSize: true,
        filename: 'analyse.html', // will be saved in project's root
    }));
}

export default defineConfig(() => {
    return {
        base: '/static/dist_vuetify_poc',
        plugins,
        resolve: {
            alias: {
                '@': resolve(__dirname, './src'),
                '@poc': resolve(__dirname, './vuetify-poc/src'),
                'stream': 'stream-browserify',
                'util': 'util/',
            },
            extensions: ['.js', '.ts', '.svg', '.vue', '.mjs'],
        },
        build: {
            outDir: resolve(__dirname, 'dist_vuetify_poc'),
            emptyOutDir: true,
            rollupOptions: {
                input: {
                    'vuetify-poc': resolve(__dirname, './index-vuetify.html'),
                },
                external: [
                    /satellite\/src\/views/,
                    /satellite\/src\/components/,
                    /satellite\/src\/router/,
                ],
            },
        },
        optimizeDeps: {
            exclude: ['* > vuetify/lib/components'],
        },
        define: {
            'process.env': {},
        },
    };
});
