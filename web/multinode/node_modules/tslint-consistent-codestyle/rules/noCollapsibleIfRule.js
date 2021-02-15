"use strict";
Object.defineProperty(exports, "__esModule", { value: true });
var tslib_1 = require("tslib");
var Lint = require("tslint");
var utils = require("tsutils");
var walker_1 = require("../src/walker");
var FAIL_MERGE_IF = "if statements can be merged";
var FAIL_MERGE_ELSE_IF = "if statement can be merged with previous else";
var Rule = (function (_super) {
    tslib_1.__extends(Rule, _super);
    function Rule() {
        return _super !== null && _super.apply(this, arguments) || this;
    }
    Rule.prototype.apply = function (sourceFile) {
        return this.applyWithWalker(new CollapsibleIfWalker(sourceFile, this.ruleName, undefined));
    };
    return Rule;
}(Lint.Rules.AbstractRule));
exports.Rule = Rule;
var CollapsibleIfWalker = (function (_super) {
    tslib_1.__extends(CollapsibleIfWalker, _super);
    function CollapsibleIfWalker() {
        return _super !== null && _super.apply(this, arguments) || this;
    }
    CollapsibleIfWalker.prototype._checkIfStatement = function (node) {
        if (node.elseStatement === undefined) {
            var then = node.thenStatement;
            if (utils.isBlockLike(then) && then.statements.length === 1)
                then = then.statements[0];
            if (utils.isIfStatement(then) && then.elseStatement === undefined)
                this.addFailure(node.getStart(this.sourceFile), then.thenStatement.pos, FAIL_MERGE_IF);
        }
        else if (utils.isBlock(node.elseStatement) &&
            node.elseStatement.statements.length === 1 &&
            utils.isIfStatement(node.elseStatement.statements[0])) {
            this.addFailure(node.elseStatement.pos - 4, node.elseStatement.statements[0].thenStatement.pos, FAIL_MERGE_ELSE_IF);
        }
    };
    return CollapsibleIfWalker;
}(walker_1.AbstractIfStatementWalker));
//# sourceMappingURL=data:application/json;base64,eyJ2ZXJzaW9uIjozLCJmaWxlIjoibm9Db2xsYXBzaWJsZUlmUnVsZS5qcyIsInNvdXJjZVJvb3QiOiIiLCJzb3VyY2VzIjpbIm5vQ29sbGFwc2libGVJZlJ1bGUudHMiXSwibmFtZXMiOltdLCJtYXBwaW5ncyI6Ijs7O0FBQ0EsNkJBQStCO0FBQy9CLCtCQUFpQztBQUVqQyx3Q0FBMEQ7QUFFMUQsSUFBTSxhQUFhLEdBQUcsNkJBQTZCLENBQUM7QUFDcEQsSUFBTSxrQkFBa0IsR0FBRywrQ0FBK0MsQ0FBQztBQUUzRTtJQUEwQixnQ0FBdUI7SUFBakQ7O0lBSUEsQ0FBQztJQUhVLG9CQUFLLEdBQVosVUFBYSxVQUF5QjtRQUNsQyxPQUFPLElBQUksQ0FBQyxlQUFlLENBQUMsSUFBSSxtQkFBbUIsQ0FBQyxVQUFVLEVBQUUsSUFBSSxDQUFDLFFBQVEsRUFBRSxTQUFTLENBQUMsQ0FBQyxDQUFDO0lBQy9GLENBQUM7SUFDTCxXQUFDO0FBQUQsQ0FBQyxBQUpELENBQTBCLElBQUksQ0FBQyxLQUFLLENBQUMsWUFBWSxHQUloRDtBQUpZLG9CQUFJO0FBTWpCO0lBQWtDLCtDQUErQjtJQUFqRTs7SUFrQkEsQ0FBQztJQWpCYSwrQ0FBaUIsR0FBM0IsVUFBNEIsSUFBb0I7UUFDNUMsSUFBSSxJQUFJLENBQUMsYUFBYSxLQUFLLFNBQVMsRUFBRTtZQUNsQyxJQUFJLElBQUksR0FBRyxJQUFJLENBQUMsYUFBYSxDQUFDO1lBQzlCLElBQUksS0FBSyxDQUFDLFdBQVcsQ0FBQyxJQUFJLENBQUMsSUFBSSxJQUFJLENBQUMsVUFBVSxDQUFDLE1BQU0sS0FBSyxDQUFDO2dCQUN2RCxJQUFJLEdBQUcsSUFBSSxDQUFDLFVBQVUsQ0FBQyxDQUFDLENBQUMsQ0FBQztZQUM5QixJQUFJLEtBQUssQ0FBQyxhQUFhLENBQUMsSUFBSSxDQUFDLElBQUksSUFBSSxDQUFDLGFBQWEsS0FBSyxTQUFTO2dCQUM3RCxJQUFJLENBQUMsVUFBVSxDQUFDLElBQUksQ0FBQyxRQUFRLENBQUMsSUFBSSxDQUFDLFVBQVUsQ0FBQyxFQUFFLElBQUksQ0FBQyxhQUFhLENBQUMsR0FBRyxFQUFFLGFBQWEsQ0FBQyxDQUFDO1NBQzlGO2FBQU0sSUFBSSxLQUFLLENBQUMsT0FBTyxDQUFDLElBQUksQ0FBQyxhQUFhLENBQUM7WUFDakMsSUFBSSxDQUFDLGFBQWEsQ0FBQyxVQUFVLENBQUMsTUFBTSxLQUFLLENBQUM7WUFDMUMsS0FBSyxDQUFDLGFBQWEsQ0FBQyxJQUFJLENBQUMsYUFBYSxDQUFDLFVBQVUsQ0FBQyxDQUFDLENBQUMsQ0FBQyxFQUFFO1lBQzlELElBQUksQ0FBQyxVQUFVLENBQ1gsSUFBSSxDQUFDLGFBQWEsQ0FBQyxHQUFHLEdBQUcsQ0FBQyxFQUNULElBQUksQ0FBQyxhQUFhLENBQUMsVUFBVSxDQUFDLENBQUMsQ0FBRSxDQUFDLGFBQWEsQ0FBQyxHQUFHLEVBQ3BFLGtCQUFrQixDQUNyQixDQUFDO1NBQ0w7SUFDTCxDQUFDO0lBQ0wsMEJBQUM7QUFBRCxDQUFDLEFBbEJELENBQWtDLGtDQUF5QixHQWtCMUQifQ==