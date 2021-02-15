"use strict";
Object.defineProperty(exports, "__esModule", { value: true });
function createForkTsCheckerWebpackPluginState() {
    return {
        report: Promise.resolve([]),
        removedFiles: [],
        watching: false,
        initialized: false,
        webpackDevServerDoneTap: undefined,
    };
}
exports.createForkTsCheckerWebpackPluginState = createForkTsCheckerWebpackPluginState;
