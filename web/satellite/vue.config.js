// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

const path = require('path');

const webpack = require('webpack');
const CompressionWebpackPlugin = require('compression-webpack-plugin');
const productionBrotliExtensions = ['js', 'css', 'ttf', 'woff', 'woff2'];
const BundleAnalyzerPlugin = require('webpack-bundle-analyzer').BundleAnalyzerPlugin;

let plugins = [
    new CompressionWebpackPlugin({
        algorithm: 'brotliCompress',
        filename: '[path][name].br',
        test: new RegExp('\\.(' + productionBrotliExtensions.join('|') + ')$'),
        threshold: 1024,
        minRatio: 0.8,
    }),
    new webpack.optimize.MinChunkSizePlugin({
        minChunkSize: 50*1024,
    }),
    new webpack.IgnorePlugin({
        contextRegExp: /bip39[\\/]src$/,
        resourceRegExp: /^\.\/wordlists\/(?!english)/,
    }),
    new webpack.ProvidePlugin({
        Buffer: ['buffer', 'Buffer'],
    }),
];

if (process.env['STORJ_DEBUG_BUNDLE_SIZE']) {
    plugins.push(new BundleAnalyzerPlugin());
}

module.exports = {
    publicPath: '/static/dist',
    productionSourceMap: false,
    parallel: true,
    lintOnSave: process.env.NODE_ENV !== 'production', // disables eslint for builds
    configureWebpack: {
        plugins: plugins,
        resolve: {
            fallback: {
                'util': require.resolve('util/'),
                'stream': require.resolve('stream-browserify'),
                'buffer': require.resolve('buffer'),
            },
        },
    },
    chainWebpack: config => {
        // Avoid breaking browser UI cache.
        config.output.chunkFilename(`js/vendors_[name]_[chunkhash].js`);
        config.output.filename(`js/app_[name]_[chunkhash].js`);

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
            .loader('vue-svg-loader')
            .options({
                svgo: {
                    plugins: [{ removeViewBox: false }],
                },
            });
    },
};
