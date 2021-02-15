'use strict';

Object.defineProperty(exports, "__esModule", {
  value: true
});
exports.warmup = exports.pitch = undefined;

var _loaderUtils = require('loader-utils');

var _loaderUtils2 = _interopRequireDefault(_loaderUtils);

var _workerPools = require('./workerPools');

function _interopRequireDefault(obj) { return obj && obj.__esModule ? obj : { default: obj }; }

function pitch() {
  const options = _loaderUtils2.default.getOptions(this) || {};
  const workerPool = (0, _workerPools.getPool)(options);
  if (!workerPool.isAbleToRun()) {
    return;
  }
  const callback = this.async();
  workerPool.run({
    loaders: this.loaders.slice(this.loaderIndex + 1).map(l => {
      return {
        loader: l.path,
        options: l.options,
        ident: l.ident
      };
    }),
    resource: this.resourcePath + (this.resourceQuery || ''),
    sourceMap: this.sourceMap,
    emitError: this.emitError,
    emitWarning: this.emitWarning,
    resolve: this.resolve,
    target: this.target,
    minimize: this.minimize,
    resourceQuery: this.resourceQuery,
    optionsContext: this.rootContext || this.options.context
  }, (err, r) => {
    if (r) {
      r.fileDependencies.forEach(d => this.addDependency(d));
      r.contextDependencies.forEach(d => this.addContextDependency(d));
    }
    if (err) {
      callback(err);
      return;
    }
    callback(null, ...r.result);
  });
}

function warmup(options, requires) {
  const workerPool = (0, _workerPools.getPool)(options);
  workerPool.warmup(requires);
}

exports.pitch = pitch;
exports.warmup = warmup; // eslint-disable-line import/prefer-default-export