"use strict";
var __importDefault = (this && this.__importDefault) || function (mod) {
    return (mod && mod.__esModule) ? mod : { "default": mod };
};
Object.defineProperty(exports, "__esModule", { value: true });
const path_1 = __importDefault(require("path"));
const getWatcher_1 = require("./getWatcher");
function getChangedFiles(compiler) {
    let changedFiles = [];
    if (compiler.modifiedFiles) {
        // webpack 5+
        changedFiles = Array.from(compiler.modifiedFiles);
    }
    else {
        const watcher = getWatcher_1.getWatcher(compiler);
        // webpack 4
        changedFiles = Object.keys((watcher && watcher.mtimes) || {});
    }
    return changedFiles.map((changedFile) => path_1.default.normalize(changedFile));
}
exports.getChangedFiles = getChangedFiles;
