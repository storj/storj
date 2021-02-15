"use strict";

Object.defineProperty(exports, "__esModule", {
  value: true
});
exports["default"] = _default;
exports.messages = exports.ruleName = void 0;

var _stylelint = require("stylelint");

var _lodash = require("lodash");

var _utils = require("../../utils");

function _createForOfIteratorHelper(o) { if (typeof Symbol === "undefined" || o[Symbol.iterator] == null) { if (Array.isArray(o) || (o = _unsupportedIterableToArray(o))) { var i = 0; var F = function F() {}; return { s: F, n: function n() { if (i >= o.length) return { done: true }; return { done: false, value: o[i++] }; }, e: function e(_e) { throw _e; }, f: F }; } throw new TypeError("Invalid attempt to iterate non-iterable instance.\nIn order to be iterable, non-array objects must have a [Symbol.iterator]() method."); } var it, normalCompletion = true, didErr = false, err; return { s: function s() { it = o[Symbol.iterator](); }, n: function n() { var step = it.next(); normalCompletion = step.done; return step; }, e: function e(_e2) { didErr = true; err = _e2; }, f: function f() { try { if (!normalCompletion && it["return"] != null) it["return"](); } finally { if (didErr) throw err; } } }; }

function _unsupportedIterableToArray(o, minLen) { if (!o) return; if (typeof o === "string") return _arrayLikeToArray(o, minLen); var n = Object.prototype.toString.call(o).slice(8, -1); if (n === "Object" && o.constructor) n = o.constructor.name; if (n === "Map" || n === "Set") return Array.from(o); if (n === "Arguments" || /^(?:Ui|I)nt(?:8|16|32)(?:Clamped)?Array$/.test(n)) return _arrayLikeToArray(o, minLen); }

function _arrayLikeToArray(arr, len) { if (len == null || len > arr.length) len = arr.length; for (var i = 0, arr2 = new Array(len); i < len; i++) { arr2[i] = arr[i]; } return arr2; }

var ruleName = (0, _utils.namespace)("no-duplicate-dollar-variables");
exports.ruleName = ruleName;

var messages = _stylelint.utils.ruleMessages(ruleName, {
  rejected: function rejected(variable) {
    return "Unexpected duplicate dollar variable ".concat(variable);
  }
});

exports.messages = messages;

function _default(value, secondaryOptions) {
  return function (root, result) {
    var validOptions = _stylelint.utils.validateOptions(result, ruleName, {
      actual: value
    }, {
      actual: secondaryOptions,
      possible: {
        ignoreInside: ["at-rule", "nested-at-rule"],
        ignoreInsideAtRules: [_lodash.isString]
      },
      optional: true
    });

    if (!validOptions) {
      return;
    }

    var vars = {};
    /**
     * Traverse the [vars] tree through the path defined by [ancestors], creating nodes as needed.
     *
     * Return the tree of the node defined by the last of [ancestors].
     */

    function getScope(ancestors) {
      var scope = vars;

      var _iterator = _createForOfIteratorHelper(ancestors),
          _step;

      try {
        for (_iterator.s(); !(_step = _iterator.n()).done;) {
          var node = _step.value;

          if (!(node in scope)) {
            scope[node] = {};
          }

          scope = scope[node];
        }
      } catch (err) {
        _iterator.e(err);
      } finally {
        _iterator.f();
      }

      return scope;
    }
    /**
     * Returns whether [variable] is declared anywhere in the scopes along the path defined by
     * [ancestors].
     */


    function isDeclared(ancestors, variable) {
      var scope = vars;

      var _iterator2 = _createForOfIteratorHelper(ancestors),
          _step2;

      try {
        for (_iterator2.s(); !(_step2 = _iterator2.n()).done;) {
          var node = _step2.value;
          scope = scope[node];
          if (scope[variable]) return true;
        }
      } catch (err) {
        _iterator2.e(err);
      } finally {
        _iterator2.f();
      }

      return false;
    }

    root.walkDecls(function (decl) {
      var isVar = decl.prop[0] === "$";
      var isInsideIgnoredAtRule = decl.parent.type === "atrule" && secondaryOptions && secondaryOptions.ignoreInside && secondaryOptions.ignoreInside === "at-rule";
      var isInsideIgnoredNestedAtRule = decl.parent.type === "atrule" && decl.parent.parent.type !== "root" && secondaryOptions && secondaryOptions.ignoreInside && secondaryOptions.ignoreInside === "nested-at-rule";
      var isInsideIgnoredSpecifiedAtRule = decl.parent.type === "atrule" && secondaryOptions && secondaryOptions.ignoreInsideAtRules && secondaryOptions.ignoreInsideAtRules.includes(decl.parent.name);

      if (!isVar || isInsideIgnoredAtRule || isInsideIgnoredNestedAtRule || isInsideIgnoredSpecifiedAtRule) {
        return;
      }

      var ancestors = [];
      var parent = decl.parent;

      while (parent !== null && parent !== undefined) {
        var parentKey = parent.toString();
        ancestors.unshift(parentKey);
        parent = parent.parent;
      }

      var scope = getScope(ancestors);

      if (isDeclared(ancestors, decl.prop)) {
        _stylelint.utils.report({
          message: messages.rejected(decl.prop),
          node: decl,
          result: result,
          ruleName: ruleName
        });
      }

      scope[decl.prop] = true;
    });
  };
}