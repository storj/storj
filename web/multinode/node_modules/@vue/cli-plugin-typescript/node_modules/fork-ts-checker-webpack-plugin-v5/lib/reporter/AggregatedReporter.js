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
const OperationCanceledError_1 = require("../error/OperationCanceledError");
const FilesChange_1 = require("./FilesChange");
/**
 * This higher order reporter aggregates too frequent getReport requests to avoid unnecessary computation.
 */
function createAggregatedReporter(reporter) {
    let pendingReportPromise;
    let queuedIndex = 0;
    let queuedChanges = [];
    const aggregatedReporter = Object.assign(Object.assign({}, reporter), { getReport: (change) => __awaiter(this, void 0, void 0, function* () {
            if (!pendingReportPromise) {
                const reportPromise = reporter.getReport(change);
                pendingReportPromise = reportPromise
                    .then(() => {
                    // remove current pending - .finally() is supported starting from Node 10
                    pendingReportPromise = undefined;
                })
                    // ignore previous errors
                    .catch(() => {
                    // remove current pending - .finally() is supported starting from Node 10
                    pendingReportPromise = undefined;
                });
                return reportPromise;
            }
            else {
                const currentIndex = ++queuedIndex;
                queuedChanges.push(change);
                return pendingReportPromise.then(() => {
                    if (queuedIndex === currentIndex) {
                        const change = FilesChange_1.aggregateFilesChanges(queuedChanges);
                        queuedChanges = [];
                        return aggregatedReporter.getReport(change);
                    }
                    else {
                        throw new OperationCanceledError_1.OperationCanceledError('getIssues canceled - new report requested.');
                    }
                });
            }
        }) });
    return aggregatedReporter;
}
exports.createAggregatedReporter = createAggregatedReporter;
