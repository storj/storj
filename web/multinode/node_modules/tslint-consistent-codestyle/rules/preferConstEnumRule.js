"use strict";
Object.defineProperty(exports, "__esModule", { value: true });
var tslib_1 = require("tslib");
var ts = require("typescript");
var Lint = require("tslint");
var tsutils_1 = require("tsutils");
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
    var seen = new Set();
    var enums = [];
    var declarations = [];
    var variables = tsutils_1.collectVariableUsage(ctx.sourceFile);
    variables.forEach(function (variable, identifier) {
        if (identifier.parent.kind !== ts.SyntaxKind.EnumDeclaration || seen.has(identifier))
            return;
        var track = {
            name: identifier.text,
            isConst: tsutils_1.hasModifier(identifier.parent.modifiers, ts.SyntaxKind.ConstKeyword),
            declarations: [],
            members: new Map(),
            canBeConst: !variable.inGlobalScope && !variable.exported,
            uses: variable.uses,
        };
        for (var _i = 0, _a = variable.declarations; _i < _a.length; _i++) {
            var declaration = _a[_i];
            seen.add(declaration);
            if (declaration.parent.kind !== ts.SyntaxKind.EnumDeclaration) {
                track.canBeConst = false;
            }
            else {
                track.declarations.push(declaration.parent);
                declarations.push({
                    track: track,
                    declaration: declaration.parent
                });
            }
        }
        enums.push(track);
    });
    declarations.sort(function (a, b) { return a.declaration.pos - b.declaration.pos; });
    for (var _i = 0, declarations_1 = declarations; _i < declarations_1.length; _i++) {
        var _a = declarations_1[_i], track = _a.track, declaration = _a.declaration;
        for (var _b = 0, _c = declaration.members; _b < _c.length; _b++) {
            var member = _c[_b];
            var isConst = track.isConst ||
                member.initializer === undefined ||
                isConstInitializer(member.initializer, track.members, findEnum);
            track.members.set(tsutils_1.getPropertyName(member.name), {
                isConst: isConst,
                stringValued: isConst && member.initializer !== undefined && isStringValued(member.initializer, track.members, findEnum),
            });
            if (!isConst)
                track.canBeConst = false;
        }
    }
    for (var _d = 0, enums_1 = enums; _d < enums_1.length; _d++) {
        var track = enums_1[_d];
        if (track.isConst || !track.canBeConst || !onlyConstUses(track))
            continue;
        for (var _e = 0, _f = track.declarations; _e < _f.length; _e++) {
            var declaration = _f[_e];
            ctx.addFailure(declaration.name.pos - 4, declaration.name.end, "Enum '" + track.name + "' can be a 'const enum'.", Lint.Replacement.appendText(declaration.name.pos - 4, 'const '));
        }
    }
    function findEnum(name) {
        for (var _i = 0, enums_2 = enums; _i < enums_2.length; _i++) {
            var track = enums_2[_i];
            if (track.name !== name.text)
                continue;
            for (var _a = 0, _b = track.uses; _a < _b.length; _a++) {
                var use = _b[_a];
                if (use.location === name)
                    return track;
            }
        }
    }
}
function onlyConstUses(track) {
    for (var _i = 0, _a = track.uses; _i < _a.length; _i++) {
        var use = _a[_i];
        if (use.domain & 2 || use.domain === 1)
            continue;
        if (use.domain & 8)
            return false;
        var parent = use.location.parent;
        switch (parent.kind) {
            default:
                return false;
            case ts.SyntaxKind.ElementAccessExpression:
                if (parent.argumentExpression === undefined ||
                    parent.argumentExpression.kind !== ts.SyntaxKind.StringLiteral)
                    return false;
                break;
            case ts.SyntaxKind.PropertyAccessExpression:
        }
    }
    return true;
}
function isConstInitializer(initializer, members, findEnum) {
    return (function isConst(node, allowStrings) {
        switch (node.kind) {
            case ts.SyntaxKind.Identifier:
                var member = members.get(node.text);
                return member !== undefined && member.isConst && (allowStrings || !member.stringValued);
            case ts.SyntaxKind.StringLiteral:
                return allowStrings;
            case ts.SyntaxKind.NumericLiteral:
                return true;
            case ts.SyntaxKind.PrefixUnaryExpression:
                return isConst(node.operand, false);
            case ts.SyntaxKind.ParenthesizedExpression:
                return isConst(node.expression, allowStrings);
        }
        if (tsutils_1.isPropertyAccessExpression(node)) {
            if (!tsutils_1.isIdentifier(node.expression))
                return false;
            var track = findEnum(node.expression);
            if (track === undefined)
                return false;
            var member = track.members.get(node.name.text);
            return member !== undefined && member.isConst && (allowStrings || !member.stringValued);
        }
        if (tsutils_1.isElementAccessExpression(node)) {
            if (!tsutils_1.isIdentifier(node.expression) ||
                node.argumentExpression === undefined ||
                !tsutils_1.isStringLiteral(node.argumentExpression))
                return false;
            var track = findEnum(node.expression);
            if (track === undefined)
                return false;
            var member = track.members.get(node.argumentExpression.text);
            return member !== undefined && member.isConst && (allowStrings || !member.stringValued);
        }
        if (tsutils_1.isBinaryExpression(node))
            return node.operatorToken.kind !== ts.SyntaxKind.AsteriskAsteriskToken &&
                node.operatorToken.kind !== ts.SyntaxKind.AmpersandAmpersandToken &&
                node.operatorToken.kind !== ts.SyntaxKind.BarBarToken &&
                !tsutils_1.isAssignmentKind(node.operatorToken.kind) &&
                isConst(node.left, false) && isConst(node.right, false);
        return false;
    })(initializer, true);
}
function isStringValued(initializer, members, findEnum) {
    return (function stringValued(node) {
        switch (node.kind) {
            case ts.SyntaxKind.ParenthesizedExpression:
                return stringValued(node.expression);
            case ts.SyntaxKind.Identifier:
                return members.get(node.text).stringValued;
            case ts.SyntaxKind.PropertyAccessExpression:
                return findEnum(node.expression)
                    .members.get(node.name.text).stringValued;
            case ts.SyntaxKind.ElementAccessExpression:
                return findEnum(node.expression)
                    .members.get(node.argumentExpression.text).stringValued;
            default:
                return true;
        }
    })(initializer);
}
//# sourceMappingURL=data:application/json;base64,eyJ2ZXJzaW9uIjozLCJmaWxlIjoicHJlZmVyQ29uc3RFbnVtUnVsZS5qcyIsInNvdXJjZVJvb3QiOiIiLCJzb3VyY2VzIjpbInByZWZlckNvbnN0RW51bVJ1bGUudHMiXSwibmFtZXMiOltdLCJtYXBwaW5ncyI6Ijs7O0FBQUEsK0JBQWlDO0FBQ2pDLDZCQUErQjtBQUMvQixtQ0FHaUI7QUFFakI7SUFBMEIsZ0NBQXVCO0lBQWpEOztJQUlBLENBQUM7SUFIVSxvQkFBSyxHQUFaLFVBQWEsVUFBeUI7UUFDbEMsT0FBTyxJQUFJLENBQUMsaUJBQWlCLENBQUMsVUFBVSxFQUFFLElBQUksQ0FBQyxDQUFDO0lBQ3BELENBQUM7SUFDTCxXQUFDO0FBQUQsQ0FBQyxBQUpELENBQTBCLElBQUksQ0FBQyxLQUFLLENBQUMsWUFBWSxHQUloRDtBQUpZLG9CQUFJO0FBeUJqQixTQUFTLElBQUksQ0FBQyxHQUEyQjtJQUNyQyxJQUFNLElBQUksR0FBRyxJQUFJLEdBQUcsRUFBaUIsQ0FBQztJQUN0QyxJQUFNLEtBQUssR0FBWSxFQUFFLENBQUM7SUFDMUIsSUFBTSxZQUFZLEdBQW1CLEVBQUUsQ0FBQztJQUN4QyxJQUFNLFNBQVMsR0FBRyw4QkFBb0IsQ0FBQyxHQUFHLENBQUMsVUFBVSxDQUFDLENBQUM7SUFDdkQsU0FBUyxDQUFDLE9BQU8sQ0FBQyxVQUFDLFFBQVEsRUFBRSxVQUFVO1FBQ25DLElBQUksVUFBVSxDQUFDLE1BQU8sQ0FBQyxJQUFJLEtBQUssRUFBRSxDQUFDLFVBQVUsQ0FBQyxlQUFlLElBQUksSUFBSSxDQUFDLEdBQUcsQ0FBQyxVQUFVLENBQUM7WUFDakYsT0FBTztRQUNYLElBQU0sS0FBSyxHQUFVO1lBQ2pCLElBQUksRUFBRSxVQUFVLENBQUMsSUFBSTtZQUNyQixPQUFPLEVBQUUscUJBQVcsQ0FBQyxVQUFVLENBQUMsTUFBTyxDQUFDLFNBQVMsRUFBRSxFQUFFLENBQUMsVUFBVSxDQUFDLFlBQVksQ0FBQztZQUM5RSxZQUFZLEVBQUUsRUFBRTtZQUNoQixPQUFPLEVBQUUsSUFBSSxHQUFH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