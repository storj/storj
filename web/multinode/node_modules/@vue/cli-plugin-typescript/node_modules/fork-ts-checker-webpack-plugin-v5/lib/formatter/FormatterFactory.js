"use strict";
Object.defineProperty(exports, "__esModule", { value: true });
const CodeFrameFormatter_1 = require("./CodeFrameFormatter");
const BasicFormatter_1 = require("./BasicFormatter");
// declare function implementation
function createFormatter(type, options) {
    switch (type) {
        case 'basic':
        case undefined:
            return BasicFormatter_1.createBasicFormatter();
        case 'codeframe':
            return CodeFrameFormatter_1.createCodeFrameFormatter(options);
        default:
            throw new Error(`Unknown "${type}" formatter. Available types are: basic, codeframe.`);
    }
}
exports.createFormatter = createFormatter;
