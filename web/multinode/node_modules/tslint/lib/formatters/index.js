"use strict";
/**
 * @license
 * Copyright 2013 Palantir Technologies, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */
Object.defineProperty(exports, "__esModule", { value: true });
exports.JUnitFormatter = exports.TapFormatter = exports.CodeFrameFormatter = exports.FileslistFormatter = exports.StylishFormatter = exports.VerboseFormatter = exports.ProseFormatter = exports.PmdFormatter = exports.JsonFormatter = void 0;
var jsonFormatter_1 = require("./jsonFormatter");
Object.defineProperty(exports, "JsonFormatter", { enumerable: true, get: function () { return jsonFormatter_1.Formatter; } });
var pmdFormatter_1 = require("./pmdFormatter");
Object.defineProperty(exports, "PmdFormatter", { enumerable: true, get: function () { return pmdFormatter_1.Formatter; } });
var proseFormatter_1 = require("./proseFormatter");
Object.defineProperty(exports, "ProseFormatter", { enumerable: true, get: function () { return proseFormatter_1.Formatter; } });
var verboseFormatter_1 = require("./verboseFormatter");
Object.defineProperty(exports, "VerboseFormatter", { enumerable: true, get: function () { return verboseFormatter_1.Formatter; } });
var stylishFormatter_1 = require("./stylishFormatter");
Object.defineProperty(exports, "StylishFormatter", { enumerable: true, get: function () { return stylishFormatter_1.Formatter; } });
var fileslistFormatter_1 = require("./fileslistFormatter");
Object.defineProperty(exports, "FileslistFormatter", { enumerable: true, get: function () { return fileslistFormatter_1.Formatter; } });
var codeFrameFormatter_1 = require("./codeFrameFormatter");
Object.defineProperty(exports, "CodeFrameFormatter", { enumerable: true, get: function () { return codeFrameFormatter_1.Formatter; } });
var tapFormatter_1 = require("./tapFormatter");
Object.defineProperty(exports, "TapFormatter", { enumerable: true, get: function () { return tapFormatter_1.Formatter; } });
var junitFormatter_1 = require("./junitFormatter");
Object.defineProperty(exports, "JUnitFormatter", { enumerable: true, get: function () { return junitFormatter_1.Formatter; } });
