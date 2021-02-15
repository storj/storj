'use strict';

Object.defineProperty(exports, "__esModule", {
  value: true
});
const stack = (err, worker, workerId) => {
  const originError = (err.stack || '').split('\n').filter(line => line.trim().startsWith('at'));

  const workerError = worker.split('\n').filter(line => line.trim().startsWith('at'));

  const diff = workerError.slice(0, workerError.length - originError.length).join('\n');

  originError.unshift(diff);
  originError.unshift(err.message);
  originError.unshift(`Thread Loader (Worker ${workerId})`);

  return originError.join('\n');
};

class WorkerError extends Error {
  constructor(err, workerId) {
    super(err);
    this.name = err.name;
    this.message = err.message;

    Error.captureStackTrace(this, this.constructor);

    this.stack = stack(err, this.stack, workerId);
  }
}

exports.default = WorkerError;