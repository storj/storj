"use strict";
Object.defineProperty(exports, "__esModule", { value: true });
var tslib_1 = require("tslib");
var ts = require("typescript");
var Lint = require("tslint");
var utils = require("tsutils");
var FAILURE_STRING = 'Modulus 2 can be replaced with & 1';
var Rule = (function (_super) {
    tslib_1.__extends(Rule, _super);
    function Rule() {
        return _super !== null && _super.apply(this, arguments) || this;
    }
    Rule.prototype.apply = function (sourceFile) {
        return this.applyWithWalker(new ReturnWalker(sourceFile, this.ruleName, undefined));
    };
    return Rule;
}(Lint.Rules.AbstractRule));
exports.Rule = Rule;
var ReturnWalker = (function (_super) {
    tslib_1.__extends(ReturnWalker, _super);
    function ReturnWalker() {
        return _super !== null && _super.apply(this, arguments) || this;
    }
    ReturnWalker.prototype.walk = function (sourceFile) {
        var _this = this;
        var cb = function (node) {
            if (utils.isBinaryExpression(node) &&
                node.operatorToken.kind === ts.SyntaxKind.PercentToken &&
                utils.isNumericLiteral(node.right) &&
                node.right.text === '2') {
                var start = node.operatorToken.getStart(sourceFile);
                _this.addFailure(start, node.right.end, FAILURE_STRING, [
                    new Lint.Replacement(start, 1, '&'),
                    new Lint.Replacement(node.right.end - 1, 1, '1'),
                ]);
            }
            return ts.forEachChild(node, cb);
        };
        return ts.forEachChild(sourceFile, cb);
    };
    return ReturnWalker;
}(Lint.AbstractWalker));
//# sourceMappingURL=data:application/json;base64,eyJ2ZXJzaW9uIjozLCJmaWxlIjoib2RkbmVzc0NoZWNrUnVsZS5qcyIsInNvdXJjZVJvb3QiOiIiLCJzb3VyY2VzIjpbIm9kZG5lc3NDaGVja1J1bGUudHMiXSwibmFtZXMiOltdLCJtYXBwaW5ncyI6Ijs7O0FBQUEsK0JBQWlDO0FBQ2pDLDZCQUErQjtBQUMvQiwrQkFBaUM7QUFFakMsSUFBTSxjQUFjLEdBQUcsb0NBQW9DLENBQUM7QUFFNUQ7SUFBMEIsZ0NBQXVCO0lBQWpEOztJQUlBLENBQUM7SUFIVSxvQkFBSyxHQUFaLFVBQWEsVUFBeUI7UUFDbEMsT0FBTyxJQUFJLENBQUMsZUFBZSxDQUFDLElBQUksWUFBWSxDQUFDLFVBQVUsRUFBRSxJQUFJLENBQUMsUUFBUSxFQUFFLFNBQVMsQ0FBQyxDQUFDLENBQUM7SUFDeEYsQ0FBQztJQUNMLFdBQUM7QUFBRCxDQUFDLEFBSkQsQ0FBMEIsSUFBSSxDQUFDLEtBQUssQ0FBQyxZQUFZLEdBSWhEO0FBSlksb0JBQUk7QUFNakI7SUFBMkIsd0NBQXlCO0lBQXBEOztJQWtCQSxDQUFDO0lBakJVLDJCQUFJLEdBQVgsVUFBWSxVQUF5QjtRQUFyQyxpQkFnQkM7UUFmRyxJQUFNLEVBQUUsR0FBRyxVQUFDLElBQWE7WUFDckIsSUFBSSxLQUFLLENBQUMsa0JBQWtCLENBQUMsSUFBSSxDQUFDO2dCQUM5QixJQUFJLENBQUMsYUFBYSxDQUFDLElBQUksS0FBSyxFQUFFLENBQUMsVUFBVSxDQUFDLFlBQVk7Z0JBQ3RELEtBQUssQ0FBQyxnQkFBZ0IsQ0FBQyxJQUFJLENBQUMsS0FBSyxDQUFDO2dCQUNsQyxJQUFJLENBQUMsS0FBSyxDQUFDLElBQUksS0FBSyxHQUFHLEVBQUU7Z0JBRXpCLElBQU0sS0FBSyxHQUFHLElBQUksQ0FBQyxhQUFhLENBQUMsUUFBUSxDQUFDLFVBQVUsQ0FBQyxDQUFDO2dCQUN0RCxLQUFJLENBQUMsVUFBVSxDQUFDLEtBQUssRUFBRSxJQUFJLENBQUMsS0FBSyxDQUFDLEdBQUcsRUFBRSxjQUFjLEVBQUU7b0JBQ25ELElBQUksSUFBSSxDQUFDLFdBQVcsQ0FBQyxLQUFLLEVBQUUsQ0FBQyxFQUFFLEdBQUcsQ0FBQztvQkFDbkMsSUFBSSxJQUFJLENBQUMsV0FBVyxDQUFDLElBQUksQ0FBQyxLQUFLLENBQUMsR0FBRyxHQUFHLENBQUMsRUFBRSxDQUFDLEVBQUUsR0FBRyxDQUFDO2lCQUNuRCxDQUFDLENBQUM7YUFDTjtZQUNELE9BQU8sRUFBRSxDQUFDLFlBQVksQ0FBQyxJQUFJLEVBQUUsRUFBRSxDQUFDLENBQUM7UUFDckMsQ0FBQyxDQUFDO1FBQ0YsT0FBTyxFQUFFLENBQUMsWUFBWSxDQUFDLFVBQVUsRUFBRSxFQUFFLENBQUMsQ0FBQztJQUMzQyxDQUFDO0lBQ0wsbUJBQUM7QUFBRCxDQUFDLEFBbEJELENBQTJCLElBQUksQ0FBQyxjQUFjLEdBa0I3QyJ9