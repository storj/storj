// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

const path = require('path');
const StyleLintPlugin = require('stylelint-webpack-plugin');

module.exports = {
    // publicPath: "/static/dist",
    productionSourceMap: false,
    parallel: true,
    configureWebpack: {
        plugins: [
            new StyleLintPlugin({
                files: ['**/*.{vue,sss,less,scss,sass}'],
                emitWarning: true,
            })
        ],
    },
    chainWebpack: config => {
        config.output.chunkFilename(`js/vendors_[hash].js`);
        config.output.filename(`js/app_[hash].js`);

        config.resolve.alias
            .set('@', path.resolve('src'));

        config
            .plugin('html')
            .tap(args => {
                args[0].template = './index.html';
                return args
            });
    }
};
