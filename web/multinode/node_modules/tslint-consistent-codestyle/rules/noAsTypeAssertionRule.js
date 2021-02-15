"use strict";
Object.defineProperty(exports, "__esModule", { value: true });
var tslib_1 = require("tslib");
var ts = require("typescript");
var Lint = require("tslint");
var tsutils_1 = require("tsutils");
var FAIL_MESSAGE = 'use <Type> instead of `as Type`';
var Rule = (function (_super) {
    tslib_1.__extends(Rule, _super);
    function Rule() {
        return _super !== null && _super.apply(this, arguments) || this;
    }
    Rule.prototype.apply = function (sourceFile) {
        return this.applyWithFunction(sourceFile, walk);
    };
    return Rule;
}(Lint.Rules.AbstractRule));
exports.Rule = Rule;
function walk(ctx) {
    if (ctx.sourceFile.languageVariant === ts.LanguageVariant.JSX)
        return;
    return ts.forEachChild(ctx.sourceFile, function cb(node) {
        var _a;
        if (tsutils_1.isAsExpression(node)) {
            var type = node.type, expression = node.expression;
            var replacement = "<" + type.getText(ctx.sourceFile) + ">";
            while (tsutils_1.isAsExpression(expression)) {
                (_a = expression, type = _a.type, expression = _a.expression);
                replacement += "<" + type.getText(ctx.sourceFile) + ">";
            }
            ctx.addFailure(type.pos - 2, node.end, FAIL_MESSAGE, [
                Lint.Replacement.appendText(expression.getStart(ctx.sourceFile), replacement),
                Lint.Replacement.deleteFromTo(expression.end, node.end),
            ]);
            return cb(expression);
        }
        return ts.forEachChild(node, cb);
    });
}
//# sourceMappingURL=data:application/json;base64,eyJ2ZXJzaW9uIjozLCJmaWxlIjoibm9Bc1R5cGVBc3NlcnRpb25SdWxlLmpzIiwic291cmNlUm9vdCI6IiIsInNvdXJjZXMiOlsibm9Bc1R5cGVBc3NlcnRpb25SdWxlLnRzIl0sIm5hbWVzIjpbXSwibWFwcGluZ3MiOiI7OztBQUFBLCtCQUFpQztBQUNqQyw2QkFBK0I7QUFDL0IsbUNBQXVDO0FBRXZDLElBQU0sWUFBWSxHQUFHLGlDQUFpQyxDQUFDO0FBRXZEO0lBQTBCLGdDQUF1QjtJQUFqRDs7SUFJQSxDQUFDO0lBSFUsb0JBQUssR0FBWixVQUFhLFVBQXlCO1FBQ2xDLE9BQU8sSUFBSSxDQUFDLGlCQUFpQixDQUFDLFVBQVUsRUFBRSxJQUFJLENBQUMsQ0FBQztJQUNwRCxDQUFDO0lBQ0wsV0FBQztBQUFELENBQUMsQUFKRCxDQUEwQixJQUFJLENBQUMsS0FBSyxDQUFDLFlBQVksR0FJaEQ7QUFKWSxvQkFBSTtBQU1qQixTQUFTLElBQUksQ0FBQyxHQUEyQjtJQUNyQyxJQUFJLEdBQUcsQ0FBQyxVQUFVLENBQUMsZUFBZSxLQUFLLEVBQUUsQ0FBQyxlQUFlLENBQUMsR0FBRztRQUN6RCxPQUFPO0lBQ1gsT0FBTyxFQUFFLENBQUMsWUFBWSxDQUFDLEdBQUcsQ0FBQyxVQUFVLEVBQUUsU0FBUyxFQUFFLENBQUMsSUFBSTs7UUFDbkQsSUFBSSx3QkFBYyxDQUFDLElBQUksQ0FBQyxFQUFFO1lBQ2pCLElBQUEsZ0JBQUksRUFBRSw0QkFBVSxDQUFTO1lBQzlCLElBQUksV0FBVyxHQUFHLE1BQUksSUFBSSxDQUFDLE9BQU8sQ0FBQyxHQUFHLENBQUMsVUFBVSxDQUFDLE1BQUcsQ0FBQztZQUN0RCxPQUFPLHdCQUFjLENBQUMsVUFBVSxDQUFDLEVBQUU7Z0JBQy9CLENBQUMsZUFBK0IsRUFBOUIsY0FBSSxFQUFFLDBCQUFVLENBQWUsQ0FBQztnQkFDbEMsV0FBVyxJQUFJLE1BQUksSUFBSSxDQUFDLE9BQU8sQ0FBQyxHQUFHLENBQUMsVUFBVSxDQUFDLE1BQUcsQ0FBQzthQUN0RDtZQUNELEdBQUcsQ0FBQyxVQUFVLENBQUMsSUFBSSxDQUFDLEdBQUcsR0FBRyxDQUFDLEVBQUUsSUFBSSxDQUFDLEdBQUcsRUFBRSxZQUFZLEVBQUU7Z0JBQ2pELElBQUksQ0FBQyxXQUFXLENBQUMsVUFBVSxDQUFDLFVBQVUsQ0FBQyxRQUFRLENBQUMsR0FBRyxDQUFDLFVBQVUsQ0FBQyxFQUFFLFdBQVcsQ0FBQztnQkFDN0UsSUFBSSxDQUFDLFdBQVcsQ0FBQyxZQUFZLENBQUMsVUFBVSxDQUFDLEdBQUcsRUFBRSxJQUFJLENBQUMsR0FBRyxDQUFDO2FBQzFELENBQUMsQ0FBQztZQUNILE9BQU8sRUFBRSxDQUFDLFVBQVUsQ0FBQyxDQUFDO1NBQ3pCO1FBQ0QsT0FBTyxFQUFFLENBQUMsWUFBWSxDQUFDLElBQUksRUFBRSxFQUFFLENBQUMsQ0FBQztJQUNyQyxDQUFDLENBQUMsQ0FBQztBQUNQLENBQUMifQ==