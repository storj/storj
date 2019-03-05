// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

module.exports = {
    chainWebpack: config => {
      config
        .plugin('html')
        .tap(args => {
          args[0].template = './index.html'
          return args
        })
    }
};
