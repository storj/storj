'use strict';

Object.defineProperty(exports, "__esModule", {
    value: true
});
exports.patchRequire = exports.patchFs = exports.unixify = exports.util = undefined;

var _patchFs = require('./patchFs');

var _patchFs2 = _interopRequireDefault(_patchFs);

var _patchRequire = require('./patchRequire');

var _patchRequire2 = _interopRequireDefault(_patchRequire);

var _correctPath = require('./correctPath');

var _lists = require('./util/lists');

var util = _interopRequireWildcard(_lists);

function _interopRequireWildcard(obj) { if (obj && obj.__esModule) { return obj; } else { var newObj = {}; if (obj != null) { for (var key in obj) { if (Object.prototype.hasOwnProperty.call(obj, key)) newObj[key] = obj[key]; } } newObj.default = obj; return newObj; } }

function _interopRequireDefault(obj) { return obj && obj.__esModule ? obj : { default: obj }; }

exports.util = util;
exports.unixify = _correctPath.unixify;
exports.patchFs = _patchFs2.default;
exports.patchRequire = _patchRequire2.default;