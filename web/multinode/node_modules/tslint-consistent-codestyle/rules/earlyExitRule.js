"use strict";
Object.defineProperty(exports, "__esModule", { value: true });
var tslib_1 = require("tslib");
var tsutils_1 = require("tsutils");
var Lint = require("tslint");
var ts = require("typescript");
var Rule = (function (_super) {
    tslib_1.__extends(Rule, _super);
    function Rule() {
        return _super !== null && _super.apply(this, arguments) || this;
    }
    Rule.prototype.apply = function (sourceFile) {
        var options = tslib_1.__assign({ 'max-length': 2, 'ignore-constructor': false }, this.ruleArguments[0]);
        return this.applyWithFunction(sourceFile, walk, options);
    };
    return Rule;
}(Lint.Rules.AbstractRule));
exports.Rule = Rule;
function failureString(exit) {
    return "Remainder of block is inside 'if' statement. Prefer to invert the condition and '" + exit + "' early.";
}
function failureStringSmall(exit, branch) {
    return "'" + branch + "' branch is small; prefer an early '" + exit + "' to a full if-else.";
}
function failureStringAlways(exit) {
    return "Prefer an early '" + exit + "' to a full if-else.";
}
function walk(ctx) {
    var sourceFile = ctx.sourceFile, _a = ctx.options, maxLineLength = _a["max-length"], ignoreConstructor = _a["ignore-constructor"];
    return ts.forEachChild(sourceFile, function cb(node) {
        if (tsutils_1.isIfStatement(node) && (!ignoreConstructor || !isConstructorClosestFunctionScopeBoundary(node)))
            check(node);
        return ts.forEachChild(node, cb);
    });
    function check(node) {
        var exit = getExit(node);
        if (exit === undefined)
            return;
        var thenStatement = node.thenStatement, elseStatement = node.elseStatement;
        var thenSize = size(thenStatement, sourceFile);
        if (elseStatement === undefined) {
            if (isLarge(thenSize))
                fail(failureString(exit));
            return;
        }
        if (elseStatement.kind === ts.SyntaxKind.IfStatement)
            return;
        if (maxLineLength === 0)
            return fail(failureStringAlways(exit));
        var elseSize = size(elseStatement, sourceFile);
        if (isSmall(thenSize) && isLarge(elseSize)) {
            fail(failureStringSmall(exit, 'then'));
        }
        else if (isSmall(elseSize) && isLarge(thenSize)) {
            fail(failureStringSmall(exit, 'else'));
        }
        function fail(failure) {
            ctx.addFailureAt(node.getStart(sourceFile), 2, failure);
        }
    }
    function isSmall(length) {
        return length === 1;
    }
    function isLarge(length) {
        return length > maxLineLength;
    }
}
function size(node, sourceFile) {
    return tsutils_1.isBlock(node)
        ? node.statements.length === 0 ? 0 : diff(node.statements[0].getStart(sourceFile), node.statements.end, sourceFile)
        : diff(node.getStart(sourceFile), node.end, sourceFile);
}
function diff(start, end, sourceFile) {
    return ts.getLineAndCharacterOfPosition(sourceFile, end).line
        - ts.getLineAndCharacterOfPosition(sourceFile, start).line
        + 1;
}
function getExit(node) {
    var parent = node.parent;
    if (tsutils_1.isBlock(parent)) {
        var container = parent.parent;
        return tsutils_1.isCaseOrDefaultClause(container) && container.statements.length === 1
            ? getCaseClauseExit(container, parent, node)
            : isLastStatement(node, parent.statements) ? getEarlyExitKind(container) : undefined;
    }
    return tsutils_1.isCaseOrDefaultClause(parent)
        ? getCaseClauseExit(parent, parent, node)
        : getEarlyExitKind(parent);
}
function getCaseClauseExit(clause, _a, node) {
    var statements = _a.statements;
    return statements[statements.length - 1].kind === ts.SyntaxKind.BreakStatement
        ? isLastStatement(node, statements, statements.length - 2) ? 'break' : undefined
        : clause.parent.clauses[clause.parent.clauses.length - 1] === clause && isLastStatement(node, statements) ? 'break' : undefined;
}
function getEarlyExitKind(_a) {
    var kind = _a.kind;
    switch (kind) {
        case ts.SyntaxKind.FunctionDeclaration:
        case ts.SyntaxKind.FunctionExpression:
        case ts.SyntaxKind.ArrowFunction:
        case ts.SyntaxKind.MethodDeclaration:
        case ts.SyntaxKind.Constructor:
        case ts.SyntaxKind.GetAccessor:
        case ts.SyntaxKind.SetAccessor:
            return 'return';
        case ts.SyntaxKind.ForInStatement:
        case ts.SyntaxKind.ForOfStatement:
        case ts.SyntaxKind.ForStatement:
        case ts.SyntaxKind.WhileStatement:
        case ts.SyntaxKind.DoStatement:
            return 'continue';
        default:
            return;
    }
}
function isLastStatement(ifStatement, statements, i) {
    if (i === void 0) { i = statements.length - 1; }
    while (true) {
        var statement = statements[i];
        if (statement === ifStatement)
            return true;
        if (statement.kind !== ts.SyntaxKind.FunctionDeclaration)
            return false;
        if (i === 0)
            throw new Error();
        i--;
    }
}
function isConstructorClosestFunctionScopeBoundary(node) {
    var currentParent = node.parent;
    while (currentParent) {
        if (tsutils_1.isFunctionScopeBoundary(currentParent))
            return currentParent.kind === ts.SyntaxKind.Constructor;
        currentParent = currentParent.parent;
    }
    return false;
}
//# sourceMappingURL=data:application/json;base64,eyJ2ZXJzaW9uIjozLCJmaWxlIjoiZWFybHlFeGl0UnVsZS5qcyIsInNvdXJjZVJvb3QiOiIiLCJzb3VyY2VzIjpbImVhcmx5RXhpdFJ1bGUudHMiXSwibmFtZXMiOltdLCJtYXBwaW5ncyI6Ijs7O0FBQUEsbUNBQWlHO0FBQ2pHLDZCQUErQjtBQUMvQiwrQkFBaUM7QUFFakM7SUFBMEIsZ0NBQXVCO0lBQWpEOztJQVNBLENBQUM7SUFSVSxvQkFBSyxHQUFaLFVBQWEsVUFBeUI7UUFDbEMsSUFBTSxPQUFPLHNCQUNULFlBQVksRUFBRSxDQUFDLEVBQ2Ysb0JBQW9CLEVBQUUsS0FBSyxJQUN4QixJQUFJLENBQUMsYUFBYSxDQUFDLENBQUMsQ0FBQyxDQUMzQixDQUFDO1FBQ0YsT0FBTyxJQUFJLENBQUMsaUJBQWlCLENBQUMsVUFBVSxFQUFFLElBQUksRUFBRSxPQUFPLENBQUMsQ0FBQztJQUM3RCxDQUFDO0lBQ0wsV0FBQztBQUFELENBQUMsQUFURCxDQUEwQixJQUFJLENBQUMsS0FBSyxDQUFDLFlBQVksR0FTaEQ7QUFUWSxvQkFBSTtBQVdqQixTQUFTLGFBQWEsQ0FBQyxJQUFZO0lBQy9CLE9BQU8sc0ZBQW9GLElBQUksYUFBVSxDQUFDO0FBQzlHLENBQUM7QUFFRCxTQUFTLGtCQUFrQixDQUFDLElBQVksRUFBRSxNQUF1QjtJQUM3RCxPQUFPLE1BQUksTUFBTSw0Q0FBdUMsSUFBSSx5QkFBc0IsQ0FBQztBQUN2RixDQUFDO0FBRUQsU0FBUyxtQkFBbUIsQ0FBQyxJQUFZO0lBQ3JDLE9BQU8sc0JBQW9CLElBQUkseUJBQXNCLENBQUM7QUFDMUQsQ0FBQztBQU9ELFNBQVMsSUFBSSxDQUFDLEdBQStCO0lBRXJDLElBQUEsMkJBQVUsRUFDVixnQkFBaUYsRUFBdEUsZ0NBQTJCLEVBQUUsNENBQXlDLENBQzdFO0lBRVIsT0FBTyxFQUFFLENBQUMsWUFBWSxDQUFDLFVBQVUsRUFBRSxTQUFTLEVBQUUsQ0FBQyxJQUFJO1FBQy9DLElBQUksdUJBQWEsQ0FBQyxJQUFJLENBQUMsSUFBSSxDQUFDLENBQUMs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