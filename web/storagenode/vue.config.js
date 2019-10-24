// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

const path = require('path');
const StyleLintPlugin = require('stylelint-webpack-plugin');

module.exports = {
    publicPath: "/static/dist",
    productionSourceMap: false,
    parallel: true,
    configureWebpack: {
        plugins: [
            new StyleLintPlugin({
                files: ['**/*.{vue,sss,less,scss,sass}'],
            })
        ],
    },
    chainWebpack: config => {
        config.output.chunkFilename(`js/vendors.js`);
        config.output.filename(`js/app.js`);

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
