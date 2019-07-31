// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

const path = require('path');

module.exports = {
    publicPath: "/static/dist",
    productionSourceMap: false,
    parallel: true,
    chainWebpack: config => {
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
