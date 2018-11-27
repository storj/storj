// Copyright (C) 2018 Storj Labs, Inc.
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
}