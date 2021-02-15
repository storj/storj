"use strict";
Object.defineProperty(exports, "__esModule", { value: true });
var tslib_1 = require("tslib");
var ts = require("typescript");
var Lint = require("tslint");
var tsutils_1 = require("tsutils");
var utils_1 = require("../src/utils");
var OPTION_ALLOW_DESTRUCTURING = 'allow-destructuring';
var Rule = (function (_super) {
    tslib_1.__extends(Rule, _super);
    function Rule() {
        return _super !== null && _super.apply(this, arguments) || this;
    }
    Rule.prototype.apply = function (sourceFile) {
        return this.applyWithFunction(sourceFile, walk, {
            allowDestructuring: this.ruleArguments.indexOf(OPTION_ALLOW_DESTRUCTURING) !== -1,
        });
    };
    return Rule;
}(Lint.Rules.AbstractRule));
exports.Rule = Rule;
function walk(ctx) {
    var variables;
    return ts.forEachChild(ctx.sourceFile, cbNode, cbNodeArray);
    function isUnused(node) {
        if (variables === undefined)
            variables = tsutils_1.collectVariableUsage(ctx.sourceFile);
        return variables.get(node).uses.length === 1;
    }
    function cbNode(node) {
        return ts.forEachChild(node, cbNode, cbNodeArray);
    }
    function cbNodeArray(nodes) {
        if (nodes.length === 0)
            return;
        ts.forEachChild(nodes[0], cbNode, cbNodeArray);
        for (var i = 1; i < nodes.length; ++i) {
            var node = nodes[i];
            if (tsutils_1.isReturnStatement(node)) {
                if (node.expression === undefined)
                    continue;
                if (!tsutils_1.isIdentifier(node.expression)) {
                    ts.forEachChild(node.expression, cbNode, cbNodeArray);
                    continue;
                }
                var previous = nodes[i - 1];
                if (tsutils_1.isVariableStatement(previous) && declaresVariable(previous, node.expression.text, isUnused, ctx.options))
                    ctx.addFailureAtNode(node.expression, "don't declare variable " + node.expression.text + " to return it immediately");
            }
            else {
                ts.forEachChild(node, cbNode, cbNodeArray);
            }
        }
    }
}
function declaresVariable(statement, name, isUnused, options) {
    var declarations = statement.declarationList.declarations;
    var lastDeclaration = declarations[declarations.length - 1].name;
    if (lastDeclaration.kind === ts.SyntaxKind.Identifier)
        return lastDeclaration.text === name && isUnused(lastDeclaration);
    return !options.allowDestructuring && isSimpleDestructuringForName(lastDeclaration, name, isUnused);
}
function isSimpleDestructuringForName(pattern, name, isUnused) {
    var identifiersSeen = new Set();
    var inArray = 0;
    var dependsOnVar = 0;
    return recur(pattern) === true;
    function recur(p) {
        if (p.kind === ts.SyntaxKind.ArrayBindingPattern) {
            ++inArray;
            for (var _i = 0, _a = p.elements; _i < _a.length; _i++) {
                var element = _a[_i];
                if (element.kind !== ts.SyntaxKind.OmittedExpression) {
                    var result = handleBindingElement(element);
                    if (result !== undefined)
                        return result;
                }
            }
            --inArray;
        }
        else {
            for (var _b = 0, _c = p.elements; _b < _c.length; _b++) {
                var element = _c[_b];
                var result = handleBindingElement(element);
                if (result !== undefined)
                    return result;
            }
        }
    }
    function handleBindingElement(element) {
        if (element.name.kind !== ts.SyntaxKind.Identifier) {
            if (dependsOnPrevious(element)) {
                ++dependsOnVar;
                var result = recur(element.name);
                --dependsOnVar;
                return result;
            }
            return recur(element.name);
        }
        if (element.name.text !== name)
            return void identifiersSeen.add(element.name.text);
        if (dependsOnVar !== 0)
            return false;
        if (element.dotDotDotToken) {
            if (element.parent.elements.length > 1 ||
                inArray > (element.parent.kind === ts.SyntaxKind.ArrayBindingPattern ? 1 : 0))
                return false;
        }
        else if (inArray !== 0) {
            return false;
        }
        if (element.initializer !== undefined && !utils_1.isUndefined(element.initializer))
            return false;
        return !dependsOnPrevious(element) && isUnused(element.name);
    }
    function dependsOnPrevious(element) {
        if (element.propertyName === undefined || element.propertyName.kind !== ts.SyntaxKind.ComputedPropertyName)
            return false;
        if (tsutils_1.isIdentifier(element.propertyName.expression))
            return identifiersSeen.has(element.propertyName.expression.text);
        if (tsutils_1.isLiteralExpression(element.propertyName.expression))
            return false;
        return true;
    }
}
//# sourceMappingURL=data:application/json;base64,eyJ2ZXJzaW9uIjozLCJmaWxlIjoibm9WYXJCZWZvcmVSZXR1cm5SdWxlLmpzIiwic291cmNlUm9vdCI6IiIsInNvdXJjZXMiOlsibm9WYXJCZWZvcmVSZXR1cm5SdWxlLnRzIl0sIm5hbWVzIjpbXSwibWFwcGluZ3MiOiI7OztBQUFBLCtCQUFpQztBQUNqQyw2QkFBK0I7QUFDL0IsbUNBQXVJO0FBRXZJLHNDQUEyQztBQUUzQyxJQUFNLDBCQUEwQixHQUFHLHFCQUFxQixDQUFDO0FBTXpEO0lBQTBCLGdDQUF1QjtJQUFqRDs7SUFNQSxDQUFDO0lBTFUsb0JBQUssR0FBWixVQUFhLFVBQXlCO1FBQ2xDLE9BQU8sSUFBSSxDQUFDLGlCQUFpQixDQUFDLFVBQVUsRUFBRSxJQUFJLEVBQUU7WUFDNUMsa0JBQWtCLEVBQUUsSUFBSSxDQUFDLGFBQWEsQ0FBQyxPQUFPLENBQUMsMEJBQTBCLENBQUMsS0FBSyxDQUFDLENBQUM7U0FDcEYsQ0FBQyxDQUFDO0lBQ1AsQ0FBQztJQUNMLFdBQUM7QUFBRCxDQUFDLEFBTkQsQ0FBMEIsSUFBSSxDQUFDLEtBQUssQ0FBQyxZQUFZLEdBTWhEO0FBTlksb0JBQUk7QUFRakIsU0FBUyxJQUFJLENBQUMsR0FBK0I7SUFDekMsSUFBSSxTQUF1RCxDQUFDO0lBQzVELE9BQU8sRUFBRSxDQUFDLFlBQVksQ0FBQyxHQUFHLENBQUMsVUFBVSxFQUFFLE1BQU0sRUFBRSxXQUFXLENBQUMsQ0FBQztJQUU1RCxTQUFTLFFBQVEsQ0FBQyxJQUFtQjtRQUNqQyxJQUFJLFNBQVMsS0FBSyxTQUFTO1lBQ3ZCLFNBQVMsR0FBRyw4QkFBb0IsQ0FBQyxHQUFHLENBQUMsVUFBVSxDQUFDLENBQUM7UUFDckQsT0FBTyxTQUFTLENBQUMsR0FBRyxDQUFDLElBQUksQ0FBRSxDQUFDLElBQUksQ0FBQyxNQUFNLEtBQUssQ0FBQyxDQUFDO0lBQ2xELENBQUM7SUFFRCxTQUFTLE1BQU0sQ0FBQyxJQUFhO1FBQ3pCLE9BQU8sRUFBRSxDQUFDLFlBQVksQ0FBQyxJQUFJLEVBQUUsTUFBTSxFQUFFLFdBQVcsQ0FBQyxDQUFDO0lBQ3RELENBQUM7SUFFRCxTQUFTLFdBQVcsQ0FBQyxLQUE2QjtRQUM5QyxJQUFJLEtBQUssQ0FBQyxNQUFNLEtBQUssQ0FBQztZQUNsQixPQUFPO1FBQ1gsRUFBRSxDQUFDLFlBQVksQ0FBQyxLQUFLLENBQUMsQ0FBQyxDQUFDLEVBQUUsTUFBTSxFQUFFLFdBQVcsQ0FBQyxDQUFDO1FBQy9DLEtBQUssSUFBSSxDQUFDLEdBQUcsQ0FBQyxFQUFFLENBQUMsR0FBRyxLQUFLLENBQUMsTUFBTSxFQUFFLEVBQUUsQ0FBQyxFQUFFO1lBQ25DLElBQU0sSUFBSSxHQUFHLEtBQUssQ0FBQyxDQUFDLENBQUMsQ0FBQztZQUN0QixJQUFJLDJCQUFpQixDQUFDLElBQUksQ0FBQyxFQUFFO2dCQUN6QixJQUFJLElBQUksQ0FBQyxVQUFVLEtBQUssU0FBUztvQkFDN0IsU0FBUztnQkFDYixJQUFJLENBQUMsc0JBQVksQ0FBQyxJQUFJLENBQUMsVUFBVSxDQUFDLEVBQUU7b0JBQ2hDLEVBQUUsQ0FBQyxZQUFZLENBQUMsSUFBSSxDQUFDLFVBQVUsRUFBRSxNQUFNLEVBQUUsV0FBVyxDQUFDLENBQUM7b0JBQ3RELFNBQVM7aUJBQ1o7Z0JBQ0QsSUFBTSxRQUFRLEdBQUcsS0FBSyxDQUFDLENBQUMsR0FBRyxDQUFDLENBQUMsQ0FBQztnQkFDOUIsSUFBSSw2QkFBbUIsQ0FBQyxRQUFRLENBQUMsSUFBSSxnQkFBZ0IsQ0FBQyxRQUFRLEVBQUUsSUFBSSxDQUFDLFVBQVUsQ0FBQyxJQUFJLEVBQUUsUUFBUSxFQUFFLEdBQUcsQ0FBQyxPQUFPLENBQUM7b0JBQ3hHLEdBQUcsQ0FBQyxnQkFBZ0IsQ0FBQyxJQUFJLENBQUMsVUFBVSxFQUFFLDRCQUEwQixJQUFJLENBQUMsVUFBVSxDQUFDLElBQUksOEJBQTJCLENBQUMsQ0FBQzthQUN4SDtpQkFBTTtnQkFDSCxFQUFFLENBQUMsWUFBWSxDQUFDLElBQUksRUFBRSxNQUFNLEVBQUUsV0FBVyxDQUFDLENBQUM7YUFDOUM7U0FDSjtJQUNMLENBQUM7QUFDTCxDQUFDO0FBRUQsU0FBUyxnQkFBZ0IsQ0FDckIsU0FBK0IsRUFDL0IsSUFBWSxFQUNaLFFBQTBDLEVBQzFDLE9BQWlCO0lBRWpCLElBQU0sWUFBWSxHQUFHLFNBQVMsQ0FBQyxlQUFlLENBQUMsWUFBWSxDQUFDO0lBQzVELElBQU0sZUFBZSxHQUFHLFlBQVksQ0FBQyxZQUFZLENBQUMsTUFBTSxHQUFHLENBQUMsQ0FBQyxDQUFDLElBQUksQ0FBQztJQUNuRSxJQUFJLGVBQWUsQ0FBQyxJQUFJLEtBQUssRUFBRSxDQUFDLFVBQVUsQ0FBQyxVQUFVO1FBQ2pELE9BQU8sZUFBZSxDQUFDLElBQUksS0FBSyxJQUFJLElBQUksUUFBUSxDQUFDLGVBQWUsQ0FBQyxDQUFDO0lBQ3RFLE9BQU8sQ0FBQyxPQUFPLENBQUMsa0JBQWtCLElBQUksNEJBQTRCLENBQUMsZUFBZSxFQUFFLElBQUksRUFBRSxRQUFRLENBQUMsQ0FBQztBQUN4RyxDQUFDO0FBRUQsU0FBUyw0QkFBNEIsQ0FBQyxPQUEwQixFQUFFLElBQVksRUFBRSxRQUEwQztJQUN0SCxJQUFNLGVBQWUsR0FBRyxJQUFJLEdBQUcsRUFBVSxDQUFDO0lBQzFDLElBQUksT0FBTyxHQUFHLENBQUMsQ0FBQztJQUNoQixJQUFJLFlBQVksR0FBRyxDQUFDLENBQUM7SUFFckIsT0FBTyxLQUFLLENBQUMsT0FBTyxDQUFDLEtBQUssSUFBSSxDQUFDO0lBRS9CLFNBQVMsS0FBSyxDQUFDLENBQW9CO1FBQy9CLElBQUksQ0FBQyxDQUFDLElBQUksS0FBSyxFQUFFLENBQUMsVUFBVSxDQUFDLG1CQUFtQixFQUFFO1lBQzlDLEVBQUUsT0FBTyxDQUFDO1lBQ1YsS0FBc0IsVUFBVSxFQUFWLEtBQUEsQ0FBQyxDQUFDLFFBQVEsRUFBVixjQUFVLEVBQVYsSUFBVSxFQUFFO2dCQUE3QixJQUFNLE9BQU8sU0FBQTtnQkFDZCxJQUFJLE9BQU8sQ0FBQyxJQUFJLEtBQUssRUFBRSxDQUFDLFVBQVUsQ0FBQyxpQkFBaUIsRUFBRTtvQkFDbEQsSUFBTSxNQUFNLEdBQUcsb0JBQW9CLENBQUMsT0FBTyxDQUFDLENBQUM7b0JBQzdDLElBQUksTUFBTSxLQUFLLFNBQVM7d0JBQ3BCLE9BQU8sTUFBTSxDQUFDO2lCQUNyQjthQUNKO1lBQ0QsRUFBRSxPQUFPLENBQUM7U0FDYjthQUFNO1lBQ0gsS0FBc0IsVUFBVSxFQUFWLEtBQUEsQ0FBQyxDQUFDLFFBQVEsRUFBVixjQUFVLEVBQVYsSUFBVSxFQUFFO2dCQUE3QixJQUFNLE9BQU8sU0FBQTtnQkFDZCxJQUFNLE1BQU0sR0FBRyxvQkFBb0IsQ0FBQyxPQUFPLENBQUMsQ0FBQztnQkFDN0MsSUFBSSxNQUFNLEtBQUssU0FBUztvQkFDcEIsT0FBTyxNQUFNLENBQUM7YUFDckI7U0FDSjtJQUNMLENBQUM7SUFDRCxTQUFTLG9CQUFvQixDQUFDLE9BQTBCO1FBQ3BELElBQUksT0FBTyxDQUFDLElBQUksQ0FBQyxJQUFJLEtBQUssRUFBRSxDQUFDLFVBQVUsQ0FBQyxVQUFVLEVBQUU7WUFDaEQsSUFBSSxpQkFBaUIsQ0FBQyxPQUFPLENBQUMsRUFBRTtnQkFDNUIsRUFBRSxZQUFZLENBQUM7Z0JBQ2YsSUFBTSxNQUFNLEdBQUcsS0FBSyxDQUFDLE9BQU8sQ0FBQyxJQUFJLENBQUMsQ0FBQztnQkFDbkMsRUFBRSxZQUFZLENBQUM7Z0JBQ2YsT0FBTyxNQUFNLENBQUM7YUFDakI7WUFDRCxPQUFPLEtBQUssQ0FBQyxPQUFPLENBQUMsSUFBSSxDQUFDLENBQUM7U0FDOUI7UUFDRCxJQUFJLE9BQU8sQ0FBQyxJQUFJLENBQUMsSUFBSSxLQUFLLElBQUk7WUFDMUIsT0FBTyxLQUFLLGVBQWUsQ0FBQyxHQUFHLENBQUMsT0FBTyxDQUFDLElBQUksQ0FBQyxJQUFJLENBQUMsQ0FBQztRQUN2RCxJQUFJLFlBQVksS0FBSyxDQUFDO1lBQ2xCLE9BQU8sS0FBSyxDQUFDO1FBQ2pCLElBQUksT0FBTyxDQUFDLGNBQWMsRUFBRTtZQUN4QixJQUFJLE9BQU8sQ0FBQyxNQUFPLENBQUMsUUFBUSxDQUFDLE1BQU0sR0FBRyxDQUFDO2dCQUNuQyxPQUFPLEdBQUcsQ0FBQyxPQUFPLENBQUMsTUFBTyxDQUFDLElBQUksS0FBSyxFQUFFLENBQUMsVUFBVSxDQUFDLG1CQUFtQixDQUFDLENBQUMsQ0FBQyxDQUFDLENBQUMsQ0FBQyxDQUFDLENBQUMsQ0FBQztnQkFDOUUsT0FBTyxLQUFLLENBQUM7U0FDcEI7YUFBTSxJQUFJLE9BQU8sS0FBSyxDQUFDLEVBQUU7WUFDdEIsT0FBTyxLQUFLLENBQUM7U0FDaEI7UUFDRCxJQUFJLE9BQU8sQ0FBQyxXQUFXLEtBQUssU0FBUyxJQUFJLENBQUMsbUJBQVcsQ0FBQyxPQUFPLENBQUMsV0FBVyxDQUFDO1lBQ3RFLE9BQU8sS0FBSyxDQUFDO1FBQ2pCLE9BQU8sQ0FBQyxpQkFBaUIsQ0FBQyxPQUFPLENBQUMsSUFBSSxRQUFRLENBQUMsT0FBTyxDQUFDLElBQUksQ0FBQyxDQUFDO0lBQ2pFLENBQUM7SUFDRCxTQUFTLGlCQUFpQixDQUFDLE9BQTBCO1FBQ2pELElBQUksT0FBTyxDQUFDLFlBQVksS0FBSyxTQUFTLElBQUksT0FBTyxDQUFDLFlBQVksQ0FBQyxJQUFJLEtBQUssRUFBRSxDQUFDLFVBQVUsQ0FBQyxvQkFBb0I7WUFDdEcsT0FBTyxLQUFLLENBQUM7UUFDakIsSUFBSSxzQkFBWSxDQUFDLE9BQU8sQ0FBQyxZQUFZLENBQUMsVUFBVSxDQUFDO1lBQzdDLE9BQU8sZUFBZSxDQUFDLEdBQUcsQ0FBQyxPQUFPLENBQUMsWUFBWSxDQUFDLFVBQVUsQ0FBQyxJQUFJLENBQUMsQ0FBQztRQUNyRSxJQUFJLDZCQUFtQixDQUFDLE9BQU8sQ0FBQyxZQUFZLENBQUMsVUFBVSxDQUFDO1lBQ3BELE9BQU8sS0FBSyxDQUFDO1FBQ2pCLE9BQU8sSUFBSSxDQUFDO0lBQ2hCLENBQUM7QUFDTCxDQUFDIn0=