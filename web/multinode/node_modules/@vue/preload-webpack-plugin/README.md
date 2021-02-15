@vue/preload-webpack-plugin
============
[![NPM version][npm-img]][npm-url]

This is a fork of [preload-webpack-plugin](https://github.com/GoogleChromeLabs/preload-webpack-plugin) with a number of changes:

- Uses a combination of `htmlWebpackPluginBeforeHtmlProcessing` and `htmlWebpackPluginAlterAssetTags` hooks to inject links as objects rather than strings. This allows for more flexibility when the tags need to be altered by other plugins.

- `include` option can be an object in the shape of `{ type?, chunks?, entries? }`. For example, to prefetch async chunks for a specific entry point:

  ``` js
  {
    rel: 'prefetch',
    include: {
      type: 'asyncChunks',
      entries: ['app']
    }
  }
  ```

- Added an `includeHtmlNames` option so that the plugin is only applied to a specific HTML file.

- Drops support for webpack v3.

- Drops support for Node < 6.

---

![preloads-plugin-compressor](https://cloud.githubusercontent.com/assets/110953/22451103/7700b812-e720-11e6-89e8-a6d4e3533159.png)

A Webpack plugin for automatically wiring up asynchronous (and other types) of JavaScript
chunks using `<link rel='preload'>`. This helps with lazy-loading.

Note: This is an extension plugin for [html-webpack-plugin](https://github.com/ampedandwired/html-webpack-plugin) - a plugin that
simplifies the creation of HTML files to serve your webpack bundles.

This plugin is a stop-gap until we add support for asynchronous chunk wiring to
[script-ext-html-webpack-plugin](https://github.com/numical/script-ext-html-webpack-plugin/pull/9).

Introduction
------------

[Preload](https://w3c.github.io/preload/) is a web standard aimed at improving performance
and granular loading of resources. It is a declarative fetch that can tell a browser to start fetching a
source because a developer knows the resource will be needed soon. [Preload: What is it good for?](https://www.smashingmagazine.com/2016/02/preload-what-is-it-good-for/)
is a recommended read if you haven't used the feature before.

In simple web apps, it's straight-forward to specify static paths to scripts you
would like to preload - especially if their names or locations are unlikely to change. In more complex apps,
JavaScript can be split into "chunks" (that represent routes or components) at with dynamic
names. These names can include hashes, numbers and other properties that can change with each build.

For example, `chunk.31132ae6680e598f8879.js`.

To make it easier to wire up async chunks for lazy-loading, this plugin offers a drop-in way to wire them up
using `<link rel='preload'>`.

Pre-requisites
--------------
This module requires Webpack 2.2.0 and above. It also requires that you're using
[html-webpack-plugin](https://github.com/ampedandwired/html-webpack-plugin) in your Webpack project.

Installation
---------------

First, install the package as a dependency in your package.json:

```sh
$ npm install --save-dev preload-webpack-plugin
```

Alternatively, using yarn:

```sh
yarn add -D preload-webpack-plugin
```

Usage
-----------------

Next, in your Webpack config, `require()` the preload plugin as follows:

```js
const PreloadWebpackPlugin = require('preload-webpack-plugin');
```

and finally, configure the plugin in your Webpack `plugins` array after `HtmlWebpackPlugin`:

```js
plugins: [
  new HtmlWebpackPlugin(),
  new PreloadWebpackPlugin()
]
```

When preloading files, the plugin will use different `as` attribute depends on the type of each
file. For each file ends with `.css`, the plugin will preload it with `as=style`, for each file ends
with `.woff2`, the plugin will preload it with `as=font`, while for all other files, `as=script`
will be used.

If you do not prefer to determine `as` attribute depends on suffix of filename, you can also
explicitly name it using `as`:

```javascript
plugins: [
  new HtmlWebpackPlugin(),
  new PreloadWebpackPlugin({
    rel: 'preload',
    as: 'script'
  })
]
```

In case you need more fine-grained control of the `as` atribute, you could also provide a function here.
When using it, entry name will be provided as the parameter, and function itself should return a
string for `as` attribute:

```javascript
plugins: [
  new HtmlWebpackPlugin(),
  new PreloadWebpackPlugin({
    rel: 'preload',
    as(entry) {
      if (/\.css$/.test(entry)) return 'style';
      if (/\.woff$/.test(entry)) return 'font';
      if (/\.png$/.test(entry)) return 'image';
      return 'script';
    }
  })
]
```

Notice that if `as=font` is used in preload, crossorigin will be added, otherwise the font resource
might be double fetched. Explains can be found in [this article](https://medium.com/reloading/preload-prefetch-and-priorities-in-chrome-776165961bbf).

By default, the plugin will assume async script chunks will be preloaded. This is the equivalent of:

```js
plugins: [
  new HtmlWebpackPlugin(),
  new PreloadWebpackPlugin({
    rel: 'preload',
    include: 'asyncChunks'
  })
]
```

For a project generating two async scripts with dynamically generated names, such as
`chunk.31132ae6680e598f8879.js` and `chunk.d15e7fdfc91b34bb78c4.js`, the following preloads
will be injected into the document `<head>`:

```html
<link rel="preload" as="script" href="chunk.31132ae6680e598f8879.js">
<link rel="preload" as="script" href="chunk.d15e7fdfc91b34bb78c4.js">
```

You can also configure the plugin to preload all chunks (vendor, async, normal chunks) using `include: 'all'`, or only preload initial chunks with `include: 'initial'`:

```js
plugins: [
  new HtmlWebpackPlugin(),
  new PreloadWebpackPlugin({
    rel: 'preload',
    include: 'all' // or 'initial'
  })
]
```

In case you work with named chunks, you can explicitly specify which ones to `include` by passing an array:
```js
plugins: [
  new HtmlWebpackPlugin(),
  new PreloadWebpackPlugin({
    rel: 'preload',
    include: ['home']
  })
]
```

will inject just this:

```html
<link rel="preload" as="script" href="home.31132ae6680e598f8879.js">
```

Filtering chunks
---------------------

There may be chunks that you don't want to have preloaded (sourcemaps, for example). Before preloading each chunk, this plugin checks that the file does not match any regex in the `fileBlacklist` option. The default value of this blacklist is `[/\.map/]`, meaning no sourcemaps will be preloaded. You can easily override this:

```js
new PreloadWebpackPlugin({
  fileBlacklist: [/\.whatever/]
})
```

Passing your own array will override the default, so if you want to continue filtering sourcemaps along with your own custom settings, you'll need to include the regex for sourcemaps:

```js
new PreloadWebpackPlugin({
  fileBlacklist: [/\.map/, /\.whatever/]
})
```

Resource Hints
---------------------

Should you wish to use Resource Hints (such as `prefetch`) instead of `preload`, this plugin also supports wiring those up.

Prefetch:

```js
plugins: [
  new HtmlWebpackPlugin(),
  new PreloadWebpackPlugin({
    rel: 'prefetch'
  })
]
```

For the async chunks mentioned earlier, the plugin would update your HTML to the following:

```html
<link rel="prefetch" href="chunk.31132ae6680e598f8879.js">
<link rel="prefetch" href="chunk.d15e7fdfc91b34bb78c4.js">
```

Demo
----------------------

A demo application implementing the [PRPL pattern](https://developers.google.com/web/fundamentals/performance/prpl-pattern/) with React that uses this plugin can be found in the `demo`
directory.

Support
-------

If you've found an error in this sample, please file an issue:
[https://github.com/googlechrome/preload-webpack-plugin/issues](https://github.com/googlechrome/preload-webpack-plugin/issues)

Patches are encouraged, and may be submitted by forking this project and
submitting a pull request through GitHub.

Contributing workflow
---------------------

`index.js` contains the primary source for the plugin, `test` contains tests and `demo` contains demo code.

Test the plugin:

```sh
$ npm install
$ npm run test
```

Lint the plugin:

```sh
$ npm run lint
$ npm run lint-fix # fix linting issues
```

The project is written in ES2015, but does not use a build-step. This may change depending on
any Node version support requests posted to the issue tracker.

Additional Notes
---------------------------

* Be careful not to `preload` resources a user is unlikely to need. This can waste their bandwidth.
* Use `preload` for the current session if you think a user is likely to visit the next page. There is no
  100% guarantee preloaded items will end up in the HTTP Cache and read locally beyond this session.
* If optimising for future sessions, use `prefetch` and `preconnect`. Prefetched resources are maintained
  in the HTTP Cache for at least 5 minutes (in Chrome) regardless of the resource's cachability.

Related plugins
--------------------------

* [script-ext-html-webpack-plugin](https://github.com/numical/script-ext-html-webpack-plugin) - Enhances html-webpack-plugin with options including 'async', 'defer', 'module' and preload (no async chunk support yet)
* [resource-hints-webpack-plugin](https://github.com/jantimon/resource-hints-webpack-plugin) - Automatically wires resource hints for your resources (similarly no async chunk support)

License
-------

Copyright 2017 Google, Inc.

Licensed to the Apache Software Foundation (ASF) under one or more contributor
license agreements.  See the NOTICE file distributed with this work for
additional information regarding copyright ownership.  The ASF licenses this
file to you under the Apache License, Version 2.0 (the "License"); you may not
use this file except in compliance with the License.  You may obtain a copy of
the License at

  http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS, WITHOUT
WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.  See the
License for the specific language governing permissions and limitations under
the License.

[npm-url]: https://www.npmjs.com/package/@vue/preload-webpack-plugin
[npm-img]: https://badge.fury.io/js/%40vue%2Fpreload-webpack-plugin.svg
