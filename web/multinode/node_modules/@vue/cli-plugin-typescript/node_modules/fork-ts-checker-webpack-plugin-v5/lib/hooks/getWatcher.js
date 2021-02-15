"use strict";
Object.defineProperty(exports, "__esModule", { value: true });
function getWatcher(compiler) {
    // webpack 4
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    const { watchFileSystem } = compiler;
    if (watchFileSystem) {
        return watchFileSystem.watcher || (watchFileSystem.wfs && watchFileSystem.wfs.watcher);
    }
}
exports.getWatcher = getWatcher;
