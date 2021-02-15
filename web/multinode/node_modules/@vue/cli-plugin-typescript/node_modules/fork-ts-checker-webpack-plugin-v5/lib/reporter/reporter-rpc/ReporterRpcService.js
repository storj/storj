"use strict";
var __awaiter = (this && this.__awaiter) || function (thisArg, _arguments, P, generator) {
    function adopt(value) { return value instanceof P ? value : new P(function (resolve) { resolve(value); }); }
    return new (P || (P = Promise))(function (resolve, reject) {
        function fulfilled(value) { try { step(generator.next(value)); } catch (e) { reject(e); } }
        function rejected(value) { try { step(generator["throw"](value)); } catch (e) { reject(e); } }
        function step(result) { result.done ? resolve(result.value) : adopt(result.value).then(fulfilled, rejected); }
        step((generator = generator.apply(thisArg, _arguments || [])).next());
    });
};
Object.defineProperty(exports, "__esModule", { value: true });
const rpc_1 = require("../../rpc");
const ReporterRpcProcedure_1 = require("./ReporterRpcProcedure");
function registerReporterRpcService(servicePort, reporterFactory) {
    const rpcService = rpc_1.createRpcService(servicePort);
    let reporterRegistered = false;
    const registerReporter = () => {
        rpcService.addCallHandler(ReporterRpcProcedure_1.configure, (configuration) => __awaiter(this, void 0, void 0, function* () {
            rpcService.removeCallHandler(ReporterRpcProcedure_1.configure);
            const reporter = reporterFactory(configuration);
            rpcService.addCallHandler(ReporterRpcProcedure_1.getIssues, reporter.getReport);
        }));
    };
    const unregisterReporter = () => {
        rpcService.removeCallHandler(ReporterRpcProcedure_1.configure);
        rpcService.removeCallHandler(ReporterRpcProcedure_1.getIssues);
    };
    return {
        isOpen: () => rpcService.isOpen() && reporterRegistered,
        open: () => __awaiter(this, void 0, void 0, function* () {
            if (!rpcService.isOpen()) {
                yield rpcService.open();
            }
            if (!reporterRegistered) {
                registerReporter();
                reporterRegistered = true;
            }
        }),
        close: () => __awaiter(this, void 0, void 0, function* () {
            if (reporterRegistered) {
                unregisterReporter();
                reporterRegistered = false;
            }
            if (rpcService.isOpen()) {
                yield rpcService.close();
            }
        }),
    };
}
exports.registerReporterRpcService = registerReporterRpcService;
