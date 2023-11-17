// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

import { fileURLToPath, URL } from 'node:url';

import vue from '@vitejs/plugin-vue';
import vuetify, { transformAssetUrls } from 'vite-plugin-vuetify';
import { defineConfig } from 'vite';

import vuetifyThemeCSS from './vitePlugins/vuetifyThemeCSS';
import papaParseWorker from './vitePlugins/papaParseWorker';

// https://vitejs.dev/config/
export default defineConfig({
    base: '/static/dist_vuetify_poc',
    plugins: [
        vue({
            template: { transformAssetUrls },
        }),
        // https://github.com/vuetifyjs/vuetify-loader/tree/next/packages/vite-plugin
        vuetify({
            autoImport: true,
            styles: {
                configFile: 'vuetify-poc/src/styles/settings.scss',
            },
        }),
        vuetifyThemeCSS(),
        papaParseWorker(),
    ],
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
        rollupOptions: {
            input: {
                'vuetify-poc': fileURLToPath(new URL('./index-vuetify.html', import.meta.url)),
            },
        },
    },
    server: {
        port: 3000,
    },
});
