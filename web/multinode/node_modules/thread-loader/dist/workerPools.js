'use strict';

Object.defineProperty(exports, "__esModule", {
  value: true
});
exports.getPool = undefined;

var _os = require('os');

var _os2 = _interopRequireDefault(_os);

var _WorkerPool = require('./WorkerPool');

var _WorkerPool2 = _interopRequireDefault(_WorkerPool);

function _interopRequireDefault(obj) { return obj && obj.__esModule ? obj : { default: obj }; }

const workerPools = Object.create(null);

function calculateNumberOfWorkers() {
  // There are situations when this call will return undefined so
  // we are fallback here to 1.
  // More info on: https://github.com/nodejs/node/issues/19022
  const cpus = _os2.default.cpus() || { length: 1 };

  return Math.max(1, cpus.length - 1);
}

function getPool(options) {
  const workerPoolOptions = {
    name: options.name || '',
    numberOfWorkers: options.workers || calculateNumberOfWorkers(),
    workerNodeArgs: options.workerNodeArgs,
    workerParallelJobs: options.workerParallelJobs || 20,
    poolTimeout: options.poolTimeout || 500,
    poolParallelJobs: options.poolParallelJobs || 200,
    poolRespawn: options.poolRespawn || false
  };
  const tpKey = JSON.stringify(workerPoolOptions);
  workerPools[tpKey] = workerPools[tpKey] || new _WorkerPool2.default(workerPoolOptions);
  const workerPool = workerPools[tpKey];
  return workerPool;
}

exports.getPool = getPool; // eslint-disable-line import/prefer-default-export