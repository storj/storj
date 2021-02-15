"use strict";
Object.defineProperty(exports, "__esModule", { value: true });
var tslib_1 = require("tslib");
var ts = require("typescript");
var Lint = require("tslint");
var FAIL_MESSAGE = "shorthand properties should come first";
var Rule = (function (_super) {
    tslib_1.__extends(Rule, _super);
    function Rule() {
        return _super !== null && _super.apply(this, arguments) || this;
    }
    Rule.prototype.apply = function (sourceFile) {
        return this.applyWithWalker(new ObjectWalker(sourceFile, this.ruleName, undefined));
    };
    return Rule;
}(Lint.Rules.AbstractRule));
exports.Rule = Rule;
var ObjectWalker = (function (_super) {
    tslib_1.__extends(ObjectWalker, _super);
    function ObjectWalker() {
        return _super !== null && _super.apply(this, arguments) || this;
    }
    ObjectWalker.prototype.walk = function (sourceFile) {
        var _this = this;
        var cb = function (node) {
            if (node.kind === ts.SyntaxKind.ObjectLiteralExpression)
                _this._checkObjectLiteral(node);
            return ts.forEachChild(node, cb);
        };
        return ts.forEachChild(sourceFile, cb);
    };
    ObjectWalker.prototype._checkObjectLiteral = function (node) {
        var seenRegularProperty = false;
        for (var _i = 0, _a = node.properties; _i < _a.length; _i++) {
            var property = _a[_i];
            if (property.kind === ts.SyntaxKind.PropertyAssignment) {
                seenRegularProperty = true;
            }
            else if (property.kind === ts.SyntaxKind.SpreadAssignment) {
                seenRegularProperty = false;
            }
            else if (seenRegularProperty && property.kind === ts.SyntaxKind.ShorthandPropertyAssignment) {
                this.addFailureAtNode(property, FAIL_MESSAGE);
            }
        }
    };
    return ObjectWalker;
}(Lint.AbstractWalker));
//# sourceMappingURL=data:application/json;base64,eyJ2ZXJzaW9uIjozLCJmaWxlIjoib2JqZWN0U2hvcnRoYW5kUHJvcGVydGllc0ZpcnN0UnVsZS5qcyIsInNvdXJjZVJvb3QiOiIiLCJzb3VyY2VzIjpbIm9iamVjdFNob3J0aGFuZFByb3BlcnRpZXNGaXJzdFJ1bGUudHMiXSwibmFtZXMiOltdLCJtYXBwaW5ncyI6Ijs7O0FBQUEsK0JBQWlDO0FBQ2pDLDZCQUErQjtBQUUvQixJQUFNLFlBQVksR0FBRyx3Q0FBd0MsQ0FBQztBQUU5RDtJQUEwQixnQ0FBdUI7SUFBakQ7O0lBSUEsQ0FBQztJQUhVLG9CQUFLLEdBQVosVUFBYSxVQUF5QjtRQUNsQyxPQUFPLElBQUksQ0FBQyxlQUFlLENBQUMsSUFBSSxZQUFZLENBQUMsVUFBVSxFQUFFLElBQUksQ0FBQyxRQUFRLEVBQUUsU0FBUyxDQUFDLENBQUMsQ0FBQztJQUN4RixDQUFDO0lBQ0wsV0FBQztBQUFELENBQUMsQUFKRCxDQUEwQixJQUFJLENBQUMsS0FBSyxDQUFDLFlBQVksR0FJaEQ7QUFKWSxvQkFBSTtBQU1qQjtJQUEyQix3Q0FBeUI7SUFBcEQ7O0lBc0JBLENBQUM7SUFyQlUsMkJBQUksR0FBWCxVQUFZLFVBQXlCO1FBQXJDLGlCQU9DO1FBTkcsSUFBTSxFQUFFLEdBQUcsVUFBQyxJQUFhO1lBQ3JCLElBQUksSUFBSSxDQUFDLElBQUksS0FBSyxFQUFFLENBQUMsVUFBVSxDQUFDLHVCQUF1QjtnQkFDbkQsS0FBSSxDQUFDLG1CQUFtQixDQUE2QixJQUFJLENBQUMsQ0FBQztZQUMvRCxPQUFPLEVBQUUsQ0FBQyxZQUFZLENBQUMsSUFBSSxFQUFFLEVBQUUsQ0FBQyxDQUFDO1FBQ3JDLENBQUMsQ0FBQztRQUNGLE9BQU8sRUFBRSxDQUFDLFlBQVksQ0FBQyxVQUFVLEVBQUUsRUFBRSxDQUFDLENBQUM7SUFDM0MsQ0FBQztJQUNPLDBDQUFtQixHQUEzQixVQUE0QixJQUFnQztRQUN4RCxJQUFJLG1CQUFtQixHQUFHLEtBQUssQ0FBQztRQUNoQyxLQUF1QixVQUFlLEVBQWYsS0FBQSxJQUFJLENBQUMsVUFBVSxFQUFmLGNBQWUsRUFBZixJQUFlLEVBQUU7WUFBbkMsSUFBTSxRQUFRLFNBQUE7WUFDZixJQUFJLFFBQVEsQ0FBQyxJQUFJLEtBQUssRUFBRSxDQUFDLFVBQVUsQ0FBQyxrQkFBa0IsRUFBRTtnQkFDcEQsbUJBQW1CLEdBQUcsSUFBSSxDQUFDO2FBQzlCO2lCQUFNLElBQUksUUFBUSxDQUFDLElBQUksS0FBSyxFQUFFLENBQUMsVUFBVSxDQUFDLGdCQUFnQixFQUFFO2dCQUV6RCxtQkFBbUIsR0FBRyxLQUFLLENBQUM7YUFDL0I7aUJBQU0sSUFBSSxtQkFBbUIsSUFBSSxRQUFRLENBQUMsSUFBSSxLQUFLLEVBQUUsQ0FBQyxVQUFVLENBQUMsMkJBQTJCLEVBQUU7Z0JBQzNGLElBQUksQ0FBQyxnQkFBZ0IsQ0FBQyxRQUFRLEVBQUUsWUFBWSxDQUFDLENBQUM7YUFDakQ7U0FDSjtJQUNMLENBQUM7SUFDTCxtQkFBQztBQUFELENBQUMsQUF0QkQsQ0FBMkIsSUFBSSxDQUFDLGNBQWMsR0FzQjdDIn0=