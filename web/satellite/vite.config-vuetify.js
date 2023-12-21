// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

import { fileURLToPath, URL } from 'node:url';

import vue from '@vitejs/plugin-vue';
import vuetify, { transformAssetUrls } from 'vite-plugin-vuetify';
import { defineConfig } from 'vite';
import viteCompression from 'vite-plugin-compression';

import vuetifyThemeCSS from './vitePlugins/vuetifyThemeCSS';
import papaParseWorker from './vitePlugins/papaParseWorker';

const productionBrotliExtensions = ['js', 'css', 'ttf', 'woff', 'woff2'];

const plugins = [
    vue({
        template: { transformAssetUrls },
    }),
    vuetify({
        autoImport: true,
        styles: {
            configFile: 'vuetify-poc/src/styles/settings.scss',
        },
    }),
    vuetifyThemeCSS(),
    papaParseWorker(),
];

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
    }

    return {
        base: '/static/dist_vuetify_poc',
        plugins,
        define: {
            'process.env': {},
            __UI_TYPE__: JSON.stringify('vuetify'),
        },
        resolve: {
            alias: {
                '@': fileURLToPath(new URL('./src', import.meta.url)),
                '@poc': fileURLToPath(new URL('./vuetify-poc/src', import.meta.url)),
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
            outDir: fileURLToPath(new URL('dist_vuetify_poc', import.meta.url)),
            emptyOutDir: true,
            reportCompressedSize: isProd,
            rollupOptions: {
                input: {
                    'vuetify-poc': fileURLToPath(new URL('./index-vuetify.html', import.meta.url)),
                },
            },
        },
    };
});
