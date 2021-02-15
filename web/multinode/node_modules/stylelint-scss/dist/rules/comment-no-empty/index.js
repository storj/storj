"use strict";

Object.defineProperty(exports, "__esModule", {
  value: true
});
exports["default"] = exports.messages = exports.ruleName = void 0;

var _stylelint = require("stylelint");

var _utils = require("../../utils");

var ruleName = (0, _utils.namespace)("comment-no-empty");
exports.ruleName = ruleName;

var messages = _stylelint.utils.ruleMessages(ruleName, {
  rejected: "Unexpected empty comment"
});

exports.messages = messages;

function rule(primary) {
  return function (root, result) {
    var validOptions = _stylelint.utils.validateOptions(result, ruleName, {
      actual: primary
    });

    if (!validOptions) {
      return;
    }

    root.walkComments(function (comment) {
      if (isEmptyComment(comment)) {
        _stylelint.utils.report({
          message: messages.rejected,
          node: comment,
          result: result,
          ruleName: ruleName
        });
      }
    });
  };
}

function isEmptyComment(comment) {
  return comment.text === "";
}

var _default = rule;
exports["default"] = _default;