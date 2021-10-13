// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

const path = require('path');
const webpack = require('webpack');
const CompressionWebpackPlugin = require('compression-webpack-plugin');
const WorkerPlugin = require('worker-plugin');
const productionBrotliExtensions = ['js', 'css', 'ttf', 'woff', 'woff2'];
const BundleAnalyzerPlugin = require('webpack-bundle-analyzer').BundleAnalyzerPlugin;

let plugins = [
    new CompressionWebpackPlugin({
        algorithm: 'brotliCompress',
        filename: '[path][name].br',
        test: new RegExp('\\.(' + productionBrotliExtensions.join('|') + ')$'),
        threshold: 1024,
        minRatio: 0.8
    }),
    new WorkerPlugin({
        globalObject: 'self',
    }),
    new webpack.optimize.MinChunkSizePlugin({
        minChunkSize: 50*1024,
    }),
    new webpack.IgnorePlugin(/^\.\/wordlists\/(?!english)/, /bip39[\\/]src$/),
];

if(process.env["STORJ_DEBUG_BUNDLE_SIZE"]) {
    plugins.push(new BundleAnalyzerPlugin());
}

module.exports = {
    publicPath: "/static/dist",
    productionSourceMap: false,
    parallel: true,
    lintOnSave: false, // disables eslint for builds
    configureWebpack: {
        plugins: plugins,
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
