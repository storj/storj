// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

import { fileURLToPath, URL } from 'node:url';

import vue from '@vitejs/plugin-vue';
import vuetify, { transformAssetUrls } from 'vite-plugin-vuetify';
import { defineConfig } from 'vite';

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
    ],
    define: { 'process.env': {} },
    resolve: {
        alias: {
            '@': fileURLToPath(new URL('./src', import.meta.url)),
            '@poc': fileURLToPath(new URL('./vuetify-poc/src', import.meta.url)),
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
