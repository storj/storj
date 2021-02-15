# Changelog

All notable changes to this project will be documented in this file. See [standard-version](https://github.com/conventional-changelog/standard-version) for commit guidelines.

### [2.1.1](https://github.com/webpack-contrib/stylelint-webpack-plugin/compare/v1.2.3...v2.1.1) (2020-10-14)


### Features

* support typescript ([#213](https://github.com/webpack-contrib/stylelint-webpack-plugin/issues/213)) ([b7dfa19](https://github.com/webpack-contrib/stylelint-webpack-plugin/commit/b7dfa195b7836bad7ac94a64a0c0a6163021a3e7))


### Bug Fixes

* avoiding https://github.com/mrmlnc/fast-glob/issues/158 ([#209](https://github.com/webpack-contrib/stylelint-webpack-plugin/issues/209)) ([14ae30d](https://github.com/webpack-contrib/stylelint-webpack-plugin/commit/14ae30df8a6d6b629c4e1fa647b4c6989377aec8))
* use better micromatch extglobs ([#216](https://github.com/webpack-contrib/stylelint-webpack-plugin/issues/216)) ([a70ed3d](https://github.com/webpack-contrib/stylelint-webpack-plugin/commit/a70ed3d6b6d8da90bf4dc371057cbe1433b4558d))

## [2.1.0](https://github.com/webpack-contrib/stylelint-webpack-plugin/compare/v1.2.3...v2.1.0) (2020-06-17)


### Features

* support typescript ([#213](https://github.com/webpack-contrib/stylelint-webpack-plugin/issues/213)) ([b7dfa19](https://github.com/webpack-contrib/stylelint-webpack-plugin/commit/b7dfa195b7836bad7ac94a64a0c0a6163021a3e7))

## [2.0.0](https://github.com/webpack-contrib/stylelint-webpack-plugin/compare/v1.2.3...v2.0.0) (2020-05-04)

### ⚠ BREAKING CHANGES

* minimum supported Node.js version is `10.13`
* minimum supported stylelint version is `13.0.0`

### Bug Fixes

* avoiding https://github.com/mrmlnc/fast-glob/issues/158 ([#209](https://github.com/webpack-contrib/stylelint-webpack-plugin/issues/209)) ([14ae30d](https://github.com/webpack-contrib/stylelint-webpack-plugin/commit/14ae30df8a6d6b629c4e1fa647b4c6989377aec8))

### [1.2.3](https://github.com/webpack-contrib/stylelint-webpack-plugin/compare/v1.2.2...v1.2.3) (2020-02-08)


### Performance

* require lint of stylelint only one time ([#207](https://github.com/webpack-contrib/stylelint-webpack-plugin/issues/207)) ([7e2495e](https://github.com/webpack-contrib/stylelint-webpack-plugin/commit/7e2495e6ba4d8cebb7f07cc9418020ea494670f8)) 

### [1.2.2](https://github.com/webpack-contrib/stylelint-webpack-plugin/compare/v1.2.1...v1.2.2) (2020-02-08)


### Bug Fixes

* replace back slashes on changed files ([#206](https://github.com/webpack-contrib/stylelint-webpack-plugin/issues/206)) ([7508028](https://github.com/webpack-contrib/stylelint-webpack-plugin/commit/7508028398d366c37d1a14e254baec9dc39b816c))

### [1.2.1](https://github.com/webpack-contrib/stylelint-webpack-plugin/compare/v1.2.0...v1.2.1) (2020-01-16)


### Bug Fixes

* compatibility stylelint v13 ([#204](https://github.com/webpack-contrib/stylelint-webpack-plugin/issues/204)) ([483be31](https://github.com/webpack-contrib/stylelint-webpack-plugin/commit/483be318450ec9a4f9eeb4bf1b1db203ba0c863d))

## [1.2.0](https://github.com/webpack-contrib/stylelint-webpack-plugin/compare/v1.1.2...v1.2.0) (2020-01-13)


### Features

* make possible to define official formatter as string ([#202](https://github.com/webpack-contrib/stylelint-webpack-plugin/issues/202)) ([8d6599c](https://github.com/webpack-contrib/stylelint-webpack-plugin/commit/8d6599c3f2f0e26d1515b01f6ecbafabeaa68fac))
* support stylelint v13 ([#203](https://github.com/webpack-contrib/stylelint-webpack-plugin/issues/203)) ([6fb31a3](https://github.com/webpack-contrib/stylelint-webpack-plugin/commit/6fb31a3931cb9d7cb0ce8cc99c9db28f928c82f4))

### [1.1.2](https://github.com/webpack-contrib/stylelint-webpack-plugin/compare/v1.1.1...v1.1.2) (2019-12-04)


### Bug Fixes

* support webpack 5 ([#199](https://github.com/webpack-contrib/stylelint-webpack-plugin/issues/199)) ([3d9e544](https://github.com/webpack-contrib/stylelint-webpack-plugin/commit/3d9e544f31172b7c01f4bd7c7254cfc7e38466c9))

### [1.1.1](https://github.com/webpack-contrib/stylelint-webpack-plugin/compare/v1.1.0...v1.1.1) (2019-12-01)


### Bug Fixes

* use hook `afterEmit` and emit error on catch ([17f7421](https://github.com/webpack-contrib/stylelint-webpack-plugin/commit/17f7421030e6a5b589b2cab015d9af80b868ca95))

## [1.1.0](https://github.com/webpack-contrib/stylelint-webpack-plugin/compare/v1.0.4...v1.1.0) (2019-11-18)


### Features

* support stylelint v12 ([#196](https://github.com/webpack-contrib/stylelint-webpack-plugin/issues/196)) ([aacf7ad](https://github.com/webpack-contrib/stylelint-webpack-plugin/commit/aacf7ad))

### [1.0.4](https://github.com/webpack-contrib/stylelint-webpack-plugin/compare/v1.0.3...v1.0.4) (2019-11-13)


### Bug Fixes

* hooks ([#195](https://github.com/webpack-contrib/stylelint-webpack-plugin/issues/195)) ([792fe19](https://github.com/webpack-contrib/stylelint-webpack-plugin/commit/792fe19))

### [1.0.3](https://github.com/webpack-contrib/stylelint-webpack-plugin/compare/v1.0.2...v1.0.3) (2019-10-25)


### Bug Fixes

* options variable ([#193](https://github.com/webpack-contrib/stylelint-webpack-plugin/issues/193)) ([3389aec](https://github.com/webpack-contrib/stylelint-webpack-plugin/commit/3389aec))

### [1.0.2](https://github.com/webpack-contrib/stylelint-webpack-plugin/compare/v1.0.1...v1.0.2) (2019-10-07)


### Bug Fixes

* convert back-slashes ([#186](https://github.com/webpack-contrib/stylelint-webpack-plugin/issues/186)) ([41b0f53](https://github.com/webpack-contrib/stylelint-webpack-plugin/commit/41b0f53))

### [1.0.1](https://github.com/webpack-contrib/stylelint-webpack-plugin/compare/v1.0.0...v1.0.1) (2019-09-30)


### Bug Fixes

* compiler hooks ([aca2c1d](https://github.com/webpack-contrib/stylelint-webpack-plugin/commit/aca2c1d))

## 1.0.0 (2019-09-30)

### Bug Fixes

* Handle compilation.fileTimestamps for webpack 4
* DeprecationWarning: Tapable.plugin is deprecated. Use new API on `.hooks` instead
* Update option `emitError`
* Update option `failOnError`

### Features

* Modernize project to latest defaults
* Validate options
* Support absolute paths in files array
* New option `stylelintPath`
* New option `emitWarning`
* New option `failOnWarning`
* New option `quiet`

### ⚠ BREAKING CHANGES

* Drop support for Node < 8.9.0
* Minimum supported `webpack` version is 4
* Minimum supported `stylelint` version is 9
