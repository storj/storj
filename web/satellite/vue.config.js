// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

const path = require('path');
const CompressionWebpackPlugin = require('compression-webpack-plugin');
const StyleLintPlugin = require('stylelint-webpack-plugin');
const productionGzipExtensions = ['js', 'css', 'ttf'];

module.exports = {
    publicPath: "/static/dist",
    productionSourceMap: false,
    parallel: true,
    configureWebpack: {
        plugins: [
            new CompressionWebpackPlugin({
                algorithm: 'gzip',
                test: new RegExp('\\.(' + productionGzipExtensions.join('|') + ')$'),
                threshold: 1024,
                minRatio: 0.8
            }),
            new StyleLintPlugin({
                files: ['**/*.{vue,sss,less,scss,sass}'],
                emitWarning: true,
            })
        ],
    },
    chainWebpack: config => {
        config.output.chunkFilename(`js/vendors_[name]_[hash].js`);
        config.output.filename(`js/app_[hash].js`);

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
