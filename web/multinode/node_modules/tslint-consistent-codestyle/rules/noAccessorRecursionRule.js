"use strict";
Object.defineProperty(exports, "__esModule", { value: true });
var tslib_1 = require("tslib");
var Lint = require("tslint");
var ts = require("typescript");
var tsutils_1 = require("tsutils");
var FAILURE_STRING = 'accessor recursion is not allowed';
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
    var name;
    return ctx.sourceFile.statements.forEach(function cb(node) {
        if (tsutils_1.isAccessorDeclaration(node) && node.body !== undefined) {
            var before = name;
            name = tsutils_1.getPropertyName(node.name);
            node.body.statements.forEach(cb);
            name = before;
        }
        else if (name !== undefined && tsutils_1.hasOwnThisReference(node)) {
            var before = name;
            name = undefined;
            ts.forEachChild(node, cb);
            name = before;
        }
        else if (name !== undefined && tsutils_1.isPropertyAccessExpression(node) &&
            node.expression.kind === ts.SyntaxKind.ThisKeyword && node.name.text === name) {
            ctx.addFailureAtNode(node, FAILURE_STRING);
        }
        else {
            return ts.forEachChild(node, cb);
        }
    });
}
//# sourceMappingURL=data:application/json;base64,eyJ2ZXJzaW9uIjozLCJmaWxlIjoibm9BY2Nlc3NvclJlY3Vyc2lvblJ1bGUuanMiLCJzb3VyY2VSb290IjoiIiwic291cmNlcyI6WyJub0FjY2Vzc29yUmVjdXJzaW9uUnVsZS50cyJdLCJuYW1lcyI6W10sIm1hcHBpbmdzIjoiOzs7QUFBQSw2QkFBK0I7QUFDL0IsK0JBQWlDO0FBQ2pDLG1DQUFrSDtBQUVsSCxJQUFNLGNBQWMsR0FBRyxtQ0FBbUMsQ0FBQztBQUUzRDtJQUEwQixnQ0FBdUI7SUFBakQ7O0lBSUEsQ0FBQztJQUhVLG9CQUFLLEdBQVosVUFBYSxVQUF5QjtRQUNsQyxPQUFPLElBQUksQ0FBQyxpQkFBaUIsQ0FBQyxVQUFVLEVBQUUsSUFBSSxDQUFDLENBQUM7SUFDcEQsQ0FBQztJQUNMLFdBQUM7QUFBRCxDQUFDLEFBSkQsQ0FBMEIsSUFBSSxDQUFDLEtBQUssQ0FBQyxZQUFZLEdBSWhEO0FBSlksb0JBQUk7QUFNakIsU0FBUyxJQUFJLENBQUMsR0FBMkI7SUFDckMsSUFBSSxJQUF3QixDQUFDO0lBRTdCLE9BQU8sR0FBRyxDQUFDLFVBQVUsQ0FBQyxVQUFVLENBQUMsT0FBTyxDQUFDLFNBQVMsRUFBRSxDQUFDLElBQWE7UUFDOUQsSUFBSSwrQkFBcUIsQ0FBQyxJQUFJLENBQUMsSUFBSSxJQUFJLENBQUMsSUFBSSxLQUFLLFNBQVMsRUFBRTtZQUN4RCxJQUFNLE1BQU0sR0FBRyxJQUFJLENBQUM7WUFDcEIsSUFBSSxHQUFHLHlCQUFlLENBQUMsSUFBSSxDQUFDLElBQUksQ0FBQyxDQUFDO1lBQ2xDLElBQUksQ0FBQyxJQUFJLENBQUMsVUFBVSxDQUFDLE9BQU8sQ0FBQyxFQUFFLENBQUMsQ0FBQztZQUNqQyxJQUFJLEdBQUcsTUFBTSxDQUFDO1NBQ2pCO2FBQU0sSUFBSSxJQUFJLEtBQUssU0FBUyxJQUFJLDZCQUFtQixDQUFDLElBQUksQ0FBQyxFQUFFO1lBQ3hELElBQU0sTUFBTSxHQUFHLElBQUksQ0FBQztZQUNwQixJQUFJLEdBQUcsU0FBUyxDQUFDO1lBQ2pCLEVBQUUsQ0FBQyxZQUFZLENBQUMsSUFBSSxFQUFFLEVBQUUsQ0FBQyxDQUFDO1lBQzFCLElBQUksR0FBRyxNQUFNLENBQUM7U0FDakI7YUFBTSxJQUFJLElBQUksS0FBSyxTQUFTLElBQUksb0NBQTBCLENBQUMsSUFBSSxDQUFDO1lBQ3RELElBQUksQ0FBQyxVQUFVLENBQUMsSUFBSSxLQUFLLEVBQUUsQ0FBQyxVQUFVLENBQUMsV0FBVyxJQUFJLElBQUksQ0FBQyxJQUFJLENBQUMsSUFBSSxLQUFLLElBQUksRUFBRTtZQUN0RixHQUFHLENBQUMsZ0JBQWdCLENBQUMsSUFBSSxFQUFFLGNBQWMsQ0FBQyxDQUFDO1NBQzlDO2FBQU07WUFDSCxPQUFPLEVBQUUsQ0FBQyxZQUFZLENBQUMsSUFBSSxFQUFFLEVBQUUsQ0FBQyxDQUFDO1NBQ3BDO0lBQ0wsQ0FBQyxDQUFDLENBQUM7QUFDUCxDQUFDIn0=