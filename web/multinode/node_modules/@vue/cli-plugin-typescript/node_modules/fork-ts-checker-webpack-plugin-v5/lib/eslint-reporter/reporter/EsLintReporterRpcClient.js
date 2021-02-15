"use strict";
var __importStar = (this && this.__importStar) || function (mod) {
    if (mod && mod.__esModule) return mod;
    var result = {};
    if (mod != null) for (var k in mod) if (Object.hasOwnProperty.call(mod, k)) result[k] = mod[k];
    result["default"] = mod;
    return result;
};
Object.defineProperty(exports, "__esModule", { value: true });
const path = __importStar(require("path"));
const reporter_1 = require("../../reporter");
const rpc_ipc_1 = require("../../rpc/rpc-ipc");
function createEsLintReporterRpcClient(configuration) {
    const channel = rpc_ipc_1.createRpcIpcMessageChannel(path.resolve(__dirname, './EsLintReporterRpcService.js'), configuration.memoryLimit);
    return reporter_1.createReporterRpcClient(channel, configuration);
}
exports.createEsLintReporterRpcClient = createEsLintReporterRpcClient;
