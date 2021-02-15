"use strict";
Object.defineProperty(exports, "__esModule", { value: true });
var tslib_1 = require("tslib");
var Lint = require("tslint");
var walker_1 = require("../src/walker");
var utils_1 = require("../src/utils");
var tsutils_1 = require("tsutils");
var FAIL_MESSAGE = "unnecessary else";
var Rule = (function (_super) {
    tslib_1.__extends(Rule, _super);
    function Rule() {
        return _super !== null && _super.apply(this, arguments) || this;
    }
    Rule.prototype.apply = function (sourceFile) {
        return this.applyWithWalker(new IfWalker(sourceFile, this.ruleName, undefined));
    };
    return Rule;
}(Lint.Rules.AbstractRule));
exports.Rule = Rule;
var IfWalker = (function (_super) {
    tslib_1.__extends(IfWalker, _super);
    function IfWalker() {
        return _super !== null && _super.apply(this, arguments) || this;
    }
    IfWalker.prototype._checkIfStatement = function (node) {
        var elseStatement = node.elseStatement;
        if (elseStatement !== undefined && !utils_1.isElseIf(node) && tsutils_1.endsControlFlow(node.thenStatement))
            this._reportUnnecessaryElse(elseStatement, FAIL_MESSAGE);
    };
    return IfWalker;
}(walker_1.AbstractIfStatementWalker));
//# sourceMappingURL=data:application/json;base64,eyJ2ZXJzaW9uIjozLCJmaWxlIjoibm9Vbm5lY2Vzc2FyeUVsc2VSdWxlLmpzIiwic291cmNlUm9vdCI6IiIsInNvdXJjZXMiOlsibm9Vbm5lY2Vzc2FyeUVsc2VSdWxlLnRzIl0sIm5hbWVzIjpbXSwibWFwcGluZ3MiOiI7OztBQUNBLDZCQUErQjtBQUMvQix3Q0FBMEQ7QUFDMUQsc0NBQXdDO0FBQ3hDLG1DQUEwQztBQUUxQyxJQUFNLFlBQVksR0FBRyxrQkFBa0IsQ0FBQztBQUV4QztJQUEwQixnQ0FBdUI7SUFBakQ7O0lBSUEsQ0FBQztJQUhRLG9CQUFLLEdBQVosVUFBYSxVQUF5QjtRQUNwQyxPQUFPLElBQUksQ0FBQyxlQUFlLENBQUMsSUFBSSxRQUFRLENBQUMsVUFBVSxFQUFFLElBQUksQ0FBQyxRQUFRLEVBQUUsU0FBUyxDQUFDLENBQUMsQ0FBQztJQUNsRixDQUFDO0lBQ0gsV0FBQztBQUFELENBQUMsQUFKRCxDQUEwQixJQUFJLENBQUMsS0FBSyxDQUFDLFlBQVksR0FJaEQ7QUFKWSxvQkFBSTtBQU1qQjtJQUF1QixvQ0FBK0I7SUFBdEQ7O0lBTUEsQ0FBQztJQUxXLG9DQUFpQixHQUEzQixVQUE0QixJQUFvQjtRQUN2QyxJQUFBLGtDQUFhLENBQVM7UUFDN0IsSUFBSSxhQUFhLEtBQUssU0FBUyxJQUFJLENBQUMsZ0JBQVEsQ0FBQyxJQUFJLENBQUMsSUFBSSx5QkFBZSxDQUFDLElBQUksQ0FBQyxhQUFhLENBQUM7WUFDckYsSUFBSSxDQUFDLHNCQUFzQixDQUFDLGFBQWEsRUFBRSxZQUFZLENBQUMsQ0FBQztJQUMvRCxDQUFDO0lBQ0gsZUFBQztBQUFELENBQUMsQUFORCxDQUF1QixrQ0FBeUIsR0FNL0MifQ==