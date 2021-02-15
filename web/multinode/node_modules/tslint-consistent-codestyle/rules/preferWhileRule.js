"use strict";
Object.defineProperty(exports, "__esModule", { value: true });
var tslib_1 = require("tslib");
var ts = require("typescript");
var Lint = require("tslint");
var FAIL_MESSAGE = 'Prefer `while` loops instead of `for` loops without an initializer and incrementor.';
var Rule = (function (_super) {
    tslib_1.__extends(Rule, _super);
    function Rule() {
        return _super !== null && _super.apply(this, arguments) || this;
    }
    Rule.prototype.apply = function (sourceFile) {
        return this.applyWithWalker(new ForWalker(sourceFile, this.ruleName, undefined));
    };
    return Rule;
}(Lint.Rules.AbstractRule));
exports.Rule = Rule;
var ForWalker = (function (_super) {
    tslib_1.__extends(ForWalker, _super);
    function ForWalker() {
        return _super !== null && _super.apply(this, arguments) || this;
    }
    ForWalker.prototype.walk = function (sourceFile) {
        var _this = this;
        var cb = function (node) {
            if (node.kind === ts.SyntaxKind.ForStatement)
                _this._checkForStatement(node);
            return ts.forEachChild(node, cb);
        };
        return ts.forEachChild(sourceFile, cb);
    };
    ForWalker.prototype._checkForStatement = function (node) {
        if (node.initializer === undefined && node.incrementor === undefined) {
            var start = node.getStart(this.sourceFile);
            var closeParenEnd = node.statement.pos;
            var fix = void 0;
            if (node.condition === undefined) {
                fix = Lint.Replacement.replaceFromTo(start, closeParenEnd, 'while (true)');
            }
            else {
                fix = [
                    Lint.Replacement.replaceFromTo(start, node.condition.getStart(this.sourceFile), 'while ('),
                    Lint.Replacement.deleteFromTo(node.condition.end, closeParenEnd - 1),
                ];
            }
            this.addFailure(start, closeParenEnd, FAIL_MESSAGE, fix);
        }
    };
    return ForWalker;
}(Lint.AbstractWalker));
//# sourceMappingURL=data:application/json;base64,eyJ2ZXJzaW9uIjozLCJmaWxlIjoicHJlZmVyV2hpbGVSdWxlLmpzIiwic291cmNlUm9vdCI6IiIsInNvdXJjZXMiOlsicHJlZmVyV2hpbGVSdWxlLnRzIl0sIm5hbWVzIjpbXSwibWFwcGluZ3MiOiI7OztBQUFBLCtCQUFpQztBQUNqQyw2QkFBK0I7QUFFL0IsSUFBTSxZQUFZLEdBQUcscUZBQXFGLENBQUM7QUFFM0c7SUFBMEIsZ0NBQXVCO0lBQWpEOztJQUlBLENBQUM7SUFIVSxvQkFBSyxHQUFaLFVBQWEsVUFBeUI7UUFDbEMsT0FBTyxJQUFJLENBQUMsZUFBZSxDQUFDLElBQUksU0FBUyxDQUFDLFVBQVUsRUFBRSxJQUFJLENBQUMsUUFBUSxFQUFFLFNBQVMsQ0FBQyxDQUFDLENBQUM7SUFDckYsQ0FBQztJQUNMLFdBQUM7QUFBRCxDQUFDLEFBSkQsQ0FBMEIsSUFBSSxDQUFDLEtBQUssQ0FBQyxZQUFZLEdBSWhEO0FBSlksb0JBQUk7QUFNakI7SUFBd0IscUNBQXlCO0lBQWpEOztJQTBCQSxDQUFDO0lBekJVLHdCQUFJLEdBQVgsVUFBWSxVQUF5QjtRQUFyQyxpQkFPQztRQU5HLElBQU0sRUFBRSxHQUFHLFVBQUMsSUFBYTtZQUNyQixJQUFJLElBQUksQ0FBQyxJQUFJLEtBQUssRUFBRSxDQUFDLFVBQVUsQ0FBQyxZQUFZO2dCQUN4QyxLQUFJLENBQUMsa0JBQWtCLENBQWtCLElBQUksQ0FBQyxDQUFDO1lBQ25ELE9BQU8sRUFBRSxDQUFDLFlBQVksQ0FBQyxJQUFJLEVBQUUsRUFBRSxDQUFDLENBQUM7UUFDckMsQ0FBQyxDQUFDO1FBQ0YsT0FBTyxFQUFFLENBQUMsWUFBWSxDQUFDLFVBQVUsRUFBRSxFQUFFLENBQUMsQ0FBQztJQUMzQyxDQUFDO0lBQ08sc0NBQWtCLEdBQTFCLFVBQTJCLElBQXFCO1FBQzVDLElBQUksSUFBSSxDQUFDLFdBQVcsS0FBSyxTQUFTLElBQUksSUFBSSxDQUFDLFdBQVcsS0FBSyxTQUFTLEVBQUU7WUFDbEUsSUFBTSxLQUFLLEdBQUcsSUFBSSxDQUFDLFFBQVEsQ0FBQyxJQUFJLENBQUMsVUFBVSxDQUFDLENBQUM7WUFDN0MsSUFBTSxhQUFhLEdBQUcsSUFBSSxDQUFDLFNBQVMsQ0FBQyxHQUFHLENBQUM7WUFDekMsSUFBSSxHQUFHLFNBQVUsQ0FBQztZQUNsQixJQUFJLElBQUksQ0FBQyxTQUFTLEtBQUssU0FBUyxFQUFFO2dCQUM5QixHQUFHLEdBQUcsSUFBSSxDQUFDLFdBQVcsQ0FBQyxhQUFhLENBQUMsS0FBSyxFQUFFLGFBQWEsRUFBRSxjQUFjLENBQUMsQ0FBQzthQUM5RTtpQkFBTTtnQkFDSCxHQUFHLEdBQUc7b0JBQ0YsSUFBSSxDQUFDLFdBQVcsQ0FBQyxhQUFhLENBQUMsS0FBSyxFQUFFLElBQUksQ0FBQyxTQUFTLENBQUMsUUFBUSxDQUFDLElBQUksQ0FBQyxVQUFVLENBQUMsRUFBRSxTQUFTLENBQUM7b0JBQzFGLElBQUksQ0FBQyxXQUFXLENBQUMsWUFBWSxDQUFDLElBQUksQ0FBQyxTQUFTLENBQUMsR0FBRyxFQUFFLGFBQWEsR0FBRyxDQUFDLENBQUM7aUJBQ3ZFLENBQUM7YUFDTDtZQUVELElBQUksQ0FBQyxVQUFVLENBQUMsS0FBSyxFQUFFLGFBQWEsRUFBRSxZQUFZLEVBQUUsR0FBRyxDQUFDLENBQUM7U0FDNUQ7SUFDTCxDQUFDO0lBQ0wsZ0JBQUM7QUFBRCxDQUFDLEFBMUJELENBQXdCLElBQUksQ0FBQyxjQUFjLEdBMEIxQyJ9