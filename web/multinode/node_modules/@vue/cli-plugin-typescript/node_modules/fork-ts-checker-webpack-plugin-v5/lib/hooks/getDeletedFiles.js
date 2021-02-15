"use strict";
var __importDefault = (this && this.__importDefault) || function (mod) {
    return (mod && mod.__esModule) ? mod : { "default": mod };
};
Object.defineProperty(exports, "__esModule", { value: true });
const path_1 = __importDefault(require("path"));
function getDeletedFiles(compiler, state) {
    let deletedFiles = [];
    if (compiler.removedFiles) {
        // webpack 5+
        deletedFiles = Array.from(compiler.removedFiles || []);
    }
    else {
        // webpack 4
        deletedFiles = [...state.removedFiles];
    }
    return deletedFiles.map((changedFile) => path_1.default.normalize(changedFile));
}
exports.getDeletedFiles = getDeletedFiles;
