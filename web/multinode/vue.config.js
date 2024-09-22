// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

const path = require('path');

module.exports = {
    productionSourceMap: false,
    parallel: true,

    // disables eslint for builds
    lintOnSave: process.env.NODE_ENV !== 'production',

    assetsDir: 'static',

    configureWebpack: {
        plugins: [],
    },

    chainWebpack: config => {
        config.resolve.alias
            .set('@', path.resolve('src'));

        // Disable node_modules/.cache directory usage due to permissions.
        // This is enabled by default in https://cli.vuejs.org/core-plugins/babel.html#caching.
        config.module.rule('js').use('babel-loader')
            .tap(options => Object.assign(options, { cacheDirectory: false }));

        config
            .plugin('html')
            .tap(args => {
                args[0].template = './index.html';
                return args;
            });

        const svgRule = config.module.rule('svg');
        svgRule.uses.clear();
        svgRule.type(); // Disable webpack 5 asset management.
        svgRule
            .use('vue-loader')
            .loader('vue-loader')
            .end()
            .use('vue-svg-loader')
            .loader('vue-svg-loader');
    },

    transpileDependencies: [
        'vuetify',
    ],
    css: {
        loaderOptions: {
          scss: {
            additionalData: `@import "@/styles/variables.scss";`
          }
        }
      },
};
