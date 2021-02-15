"use strict";
var __importDefault = (this && this.__importDefault) || function (mod) {
    return (mod && mod.__esModule) ? mod : { "default": mod };
};
Object.defineProperty(exports, "__esModule", { value: true });
const path_1 = __importDefault(require("path"));
function tapAfterCompileToAddDependencies(compiler, configuration) {
    compiler.hooks.afterCompile.tap('ForkTsCheckerWebpackPlugin', (compilation) => {
        if (configuration.typescript.enabled) {
            // watch tsconfig.json file
            compilation.fileDependencies.add(path_1.default.normalize(configuration.typescript.configFile));
        }
    });
}
exports.tapAfterCompileToAddDependencies = tapAfterCompileToAddDependencies;
