"use strict";
Object.defineProperty(exports, "__esModule", { value: true });
var ts = require("typescript");
var utils = require("tsutils");
function isUndefined(expression) {
    return utils.isIdentifier(expression) && expression.text === 'undefined' ||
        expression.kind === ts.SyntaxKind.VoidExpression;
}
exports.isUndefined = isUndefined;
function isElseIf(node) {
    var parent = node.parent;
    return utils.isIfStatement(parent) &&
        parent.elseStatement === node;
}
exports.isElseIf = isElseIf;
//# sourceMappingURL=data:application/json;base64,eyJ2ZXJzaW9uIjozLCJmaWxlIjoidXRpbHMuanMiLCJzb3VyY2VSb290IjoiIiwic291cmNlcyI6WyJ1dGlscy50cyJdLCJuYW1lcyI6W10sIm1hcHBpbmdzIjoiOztBQUFBLCtCQUFpQztBQUNqQywrQkFBaUM7QUFFakMsU0FBZ0IsV0FBVyxDQUFDLFVBQXlCO0lBQ2pELE9BQU8sS0FBSyxDQUFDLFlBQVksQ0FBQyxVQUFVLENBQUMsSUFBSSxVQUFVLENBQUMsSUFBSSxLQUFLLFdBQVc7UUFDcEUsVUFBVSxDQUFDLElBQUksS0FBSyxFQUFFLENBQUMsVUFBVSxDQUFDLGNBQWMsQ0FBQztBQUN6RCxDQUFDO0FBSEQsa0NBR0M7QUFFRCxTQUFnQixRQUFRLENBQUMsSUFBb0I7SUFDekMsSUFBTSxNQUFNLEdBQUcsSUFBSSxDQUFDLE1BQU8sQ0FBQztJQUM1QixPQUFPLEtBQUssQ0FBQyxhQUFhLENBQUMsTUFBTSxDQUFDO1FBQzdCLE1BQU0sQ0FBQyxhQUFhLEtBQUssSUFBSSxDQUFDO0FBQ3ZDLENBQUM7QUFKRCw0QkFJQyJ9