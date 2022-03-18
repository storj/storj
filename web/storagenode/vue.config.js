// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

const path = require('path');

module.exports = {
    productionSourceMap: false,
    parallel: true,
    lintOnSave: false, // disables eslint for builds
    assetsDir: "static",
    configureWebpack: {
        plugins: [],
    },
    chainWebpack: config => {
        config.resolve.alias
            .set('@', path.resolve('src'));

        config
            .plugin('html')
            .tap(args => {
                args[0].template = './index.html';
                return args
            });

        const svgRule = config.module.rule('svg');

        svgRule.uses.clear();

        svgRule
            .use('babel-loader')
            .loader('babel-loader')
            .end()
            .use('vue-svg-loader')
            .loader('vue-svg-loader');
    }
};
