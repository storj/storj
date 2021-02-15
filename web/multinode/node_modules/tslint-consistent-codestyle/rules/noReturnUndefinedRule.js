"use strict";
Object.defineProperty(exports, "__esModule", { value: true });
var tslib_1 = require("tslib");
var Lint = require("tslint");
var utils = require("tsutils");
var utils_1 = require("../src/utils");
var walker_1 = require("../src/walker");
var FAIL_MESSAGE = "don't return explicit undefined";
var ALLOW_VOID_EXPRESSION_OPTION = 'allow-void-expression';
var Rule = (function (_super) {
    tslib_1.__extends(Rule, _super);
    function Rule() {
        return _super !== null && _super.apply(this, arguments) || this;
    }
    Rule.prototype.apply = function (sourceFile) {
        return this.applyWithWalker(new ReturnWalker(sourceFile, this.ruleName, {
            allowVoid: this.ruleArguments.indexOf(ALLOW_VOID_EXPRESSION_OPTION) !== -1,
        }));
    };
    return Rule;
}(Lint.Rules.AbstractRule));
exports.Rule = Rule;
var ReturnWalker = (function (_super) {
    tslib_1.__extends(ReturnWalker, _super);
    function ReturnWalker() {
        return _super !== null && _super.apply(this, arguments) || this;
    }
    ReturnWalker.prototype._checkReturnStatement = function (node) {
        if (node.expression !== undefined && this._isUndefined(node.expression))
            this.addFailureAtNode(node.expression, FAIL_MESSAGE);
    };
    ReturnWalker.prototype._isUndefined = function (expression) {
        return this.options.allowVoid ? isUndefinedNotVoidExpr(expression) : utils_1.isUndefined(expression);
    };
    return ReturnWalker;
}(walker_1.AbstractReturnStatementWalker));
function isUndefinedNotVoidExpr(expression) {
    if (utils.isIdentifier(expression) && expression.text === 'undefined')
        return true;
    return utils.isVoidExpression(expression) && utils.isLiteralExpression(expression.expression);
}
//# sourceMappingURL=data:application/json;base64,eyJ2ZXJzaW9uIjozLCJmaWxlIjoibm9SZXR1cm5VbmRlZmluZWRSdWxlLmpzIiwic291cmNlUm9vdCI6IiIsInNvdXJjZXMiOlsibm9SZXR1cm5VbmRlZmluZWRSdWxlLnRzIl0sIm5hbWVzIjpbXSwibWFwcGluZ3MiOiI7OztBQUNBLDZCQUErQjtBQUMvQiwrQkFBaUM7QUFFakMsc0NBQXlDO0FBQ3pDLHdDQUE0RDtBQUU1RCxJQUFNLFlBQVksR0FBRyxpQ0FBaUMsQ0FBQztBQUN2RCxJQUFNLDRCQUE0QixHQUFHLHVCQUF1QixDQUFDO0FBTTdEO0lBQTBCLGdDQUF1QjtJQUFqRDs7SUFNQSxDQUFDO0lBTFUsb0JBQUssR0FBWixVQUFhLFVBQXlCO1FBQ2xDLE9BQU8sSUFBSSxDQUFDLGVBQWUsQ0FBQyxJQUFJLFlBQVksQ0FBQyxVQUFVLEVBQUUsSUFBSSxDQUFDLFFBQVEsRUFBRTtZQUNwRSxTQUFTLEVBQUUsSUFBSSxDQUFDLGFBQWEsQ0FBQyxPQUFPLENBQUMsNEJBQTRCLENBQUMsS0FBSyxDQUFDLENBQUM7U0FDN0UsQ0FBQyxDQUFDLENBQUM7SUFDUixDQUFDO0lBQ0wsV0FBQztBQUFELENBQUMsQUFORCxDQUEwQixJQUFJLENBQUMsS0FBSyxDQUFDLFlBQVksR0FNaEQ7QUFOWSxvQkFBSTtBQVFqQjtJQUEyQix3Q0FBdUM7SUFBbEU7O0lBU0EsQ0FBQztJQVJhLDRDQUFxQixHQUEvQixVQUFnQyxJQUF3QjtRQUNwRCxJQUFJLElBQUksQ0FBQyxVQUFVLEtBQUssU0FBUyxJQUFJLElBQUksQ0FBQyxZQUFZLENBQUMsSUFBSSxDQUFDLFVBQVUsQ0FBQztZQUNuRSxJQUFJLENBQUMsZ0JBQWdCLENBQUMsSUFBSSxDQUFDLFVBQVUsRUFBRSxZQUFZLENBQUMsQ0FBQztJQUM3RCxDQUFDO0lBRU8sbUNBQVksR0FBcEIsVUFBcUIsVUFBeUI7UUFDMUMsT0FBTyxJQUFJLENBQUMsT0FBTyxDQUFDLFNBQVMsQ0FBQyxDQUFDLENBQUMsc0JBQXNCLENBQUMsVUFBVSxDQUFDLENBQUMsQ0FBQyxDQUFDLG1CQUFXLENBQUMsVUFBVSxDQUFDLENBQUM7SUFDakcsQ0FBQztJQUNMLG1CQUFDO0FBQUQsQ0FBQyxBQVRELENBQTJCLHNDQUE2QixHQVN2RDtBQUVELFNBQVMsc0JBQXNCLENBQUMsVUFBeUI7SUFDckQsSUFBSSxLQUFLLENBQUMsWUFBWSxDQUFDLFVBQVUsQ0FBQyxJQUFJLFVBQVUsQ0FBQyxJQUFJLEtBQUssV0FBVztRQUNqRSxPQUFPLElBQUksQ0FBQztJQUNoQixPQUFPLEtBQUssQ0FBQyxnQkFBZ0IsQ0FBQyxVQUFVLENBQUMsSUFBSSxLQUFLLENBQUMsbUJBQW1CLENBQUMsVUFBVSxDQUFDLFVBQVUsQ0FBQyxDQUFDO0FBQ2xHLENBQUMifQ==