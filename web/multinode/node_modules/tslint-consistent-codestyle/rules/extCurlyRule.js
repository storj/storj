"use strict";
Object.defineProperty(exports, "__esModule", { value: true });
var tslib_1 = require("tslib");
var ts = require("typescript");
var Lint = require("tslint");
var tsutils_1 = require("tsutils");
var utils_1 = require("../src/utils");
var FAIL_MESSAGE_MISSING = "statement must be braced";
var FAIL_MESSAGE_UNNECESSARY = "unnecessary curly braces";
var OPTION_ELSE = 'else';
var OPTION_CONSISTENT = 'consistent';
var OPTION_BRACED_CHILD = 'braced-child';
var OPTION_NESTED_IF_ELSE = 'nested-if-else';
var Rule = (function (_super) {
    tslib_1.__extends(Rule, _super);
    function Rule() {
        return _super !== null && _super.apply(this, arguments) || this;
    }
    Rule.prototype.apply = function (sourceFile) {
        return this.applyWithWalker(new ExtCurlyWalker(sourceFile, this.ruleName, {
            else: this.ruleArguments.indexOf(OPTION_ELSE) !== -1,
            consistent: this.ruleArguments.indexOf(OPTION_CONSISTENT) !== -1,
            child: this.ruleArguments.indexOf(OPTION_BRACED_CHILD) !== -1,
            nestedIfElse: this.ruleArguments.indexOf(OPTION_NESTED_IF_ELSE) !== -1,
        }));
    };
    return Rule;
}(Lint.Rules.AbstractRule));
exports.Rule = Rule;
var ExtCurlyWalker = (function (_super) {
    tslib_1.__extends(ExtCurlyWalker, _super);
    function ExtCurlyWalker() {
        return _super !== null && _super.apply(this, arguments) || this;
    }
    ExtCurlyWalker.prototype.walk = function (sourceFile) {
        var _this = this;
        var cb = function (node) {
            if (tsutils_1.isIterationStatement(node)) {
                _this._checkLoop(node);
            }
            else if (tsutils_1.isIfStatement(node)) {
                _this._checkIfStatement(node);
            }
            return ts.forEachChild(node, cb);
        };
        return ts.forEachChild(sourceFile, cb);
    };
    ExtCurlyWalker.prototype._checkLoop = function (node) {
        if (this._needsBraces(node.statement)) {
            if (node.statement.kind !== ts.SyntaxKind.Block)
                this.addFailureAtNode(node.statement, FAIL_MESSAGE_MISSING);
        }
        else if (node.statement.kind === ts.SyntaxKind.Block) {
            this._reportUnnecessary(node.statement);
        }
    };
    ExtCurlyWalker.prototype._checkIfStatement = function (node) {
        var _a = this._ifStatementNeedsBraces(node), then = _a[0], otherwise = _a[1];
        if (then) {
            if (node.thenStatement.kind !== ts.SyntaxKind.Block)
                this.addFailureAtNode(node.thenStatement, FAIL_MESSAGE_MISSING);
        }
        else if (node.thenStatement.kind === ts.SyntaxKind.Block) {
            this._reportUnnecessary(node.thenStatement);
        }
        if (otherwise) {
            if (node.elseStatement !== undefined &&
                node.elseStatement.kind !== ts.SyntaxKind.Block && node.elseStatement.kind !== ts.SyntaxKind.IfStatement)
                this.addFailureAtNode(node.elseStatement, FAIL_MESSAGE_MISSING);
        }
        else if (node.elseStatement !== undefined && node.elseStatement.kind === ts.SyntaxKind.Block) {
            this._reportUnnecessary(node.elseStatement);
        }
    };
    ExtCurlyWalker.prototype._needsBraces = function (node, allowIfElse) {
        if (tsutils_1.isBlock(node))
            return node.statements.length !== 1 || this._needsBraces(node.statements[0], allowIfElse);
        if (!allowIfElse && this.options.nestedIfElse && tsutils_1.isIfStatement(node) && node.elseStatement !== undefined)
            return true;
        if (!this.options.child)
            return false;
        if (tsutils_1.isIfStatement(node)) {
            var result = this._ifStatementNeedsBraces(node);
            return result[0] || result[1];
        }
        if (tsutils_1.isIterationStatement(node) || tsutils_1.isLabeledStatement(node))
            return this._needsBraces(node.statement);
        return node.kind === ts.SyntaxKind.SwitchStatement || node.kind === ts.SyntaxKind.TryStatement;
    };
    ExtCurlyWalker.prototype._ifStatementNeedsBraces = function (node, excludeElse) {
        if (this.options.else) {
            if (node.elseStatement !== undefined || utils_1.isElseIf(node))
                return [true, true];
        }
        else if (this.options.consistent) {
            if (this._needsBraces(node.thenStatement) ||
                !excludeElse && node.elseStatement !== undefined &&
                    (tsutils_1.isIfStatement(node.elseStatement)
                        ? this._ifStatementNeedsBraces(node.elseStatement)[0]
                        : this._needsBraces(node.elseStatement, true)))
                return [true, true];
            if (utils_1.isElseIf(node) && this._ifStatementNeedsBraces(node.parent, true)[0])
                return [true, true];
        }
        if (node.elseStatement !== undefined) {
            var statement = unwrapBlock(node.thenStatement);
            return [
                tsutils_1.isIfStatement(statement) && statement.elseStatement === undefined || this._needsBraces(statement),
                !excludeElse && this._needsBraces(node.elseStatement, true),
            ];
        }
        return [this._needsBraces(node.thenStatement), false];
    };
    ExtCurlyWalker.prototype._reportUnnecessary = function (block) {
        var closeBrace = block.getChildAt(2, this.sourceFile);
        var nextTokenStart = tsutils_1.getNextToken(closeBrace, this.sourceFile).getStart(this.sourceFile);
        var closeFix = tsutils_1.isSameLine(this.sourceFile, closeBrace.end, nextTokenStart)
            ? Lint.Replacement.deleteFromTo(closeBrace.end - 1, nextTokenStart)
            : Lint.Replacement.deleteFromTo(block.statements.end, block.end);
        this.addFailure(block.statements.pos - 1, block.end, FAIL_MESSAGE_UNNECESSARY, [
            Lint.Replacement.deleteFromTo(block.pos, block.statements.pos),
            closeFix,
        ]);
    };
    return ExtCurlyWalker;
}(Lint.AbstractWalker));
function unwrapBlock(node) {
    while (tsutils_1.isBlock(node) && node.statements.length === 1)
        node = node.statements[0];
    return node;
}
//# sourceMappingURL=data:application/json;base64,eyJ2ZXJzaW9uIjozLCJmaWxlIjoiZXh0Q3VybHlSdWxlLmpzIiwic291cmNlUm9vdCI6IiIsInNvdXJjZXMiOlsiZXh0Q3VybHlSdWxlLnRzIl0sIm5hbWVzIjpbXSwibWFwcGluZ3MiOiI7OztBQUFBLCtCQUFpQztBQUNqQyw2QkFBK0I7QUFDL0IsbUNBQXFIO0FBQ3JILHNDQUF3QztBQUV4QyxJQUFNLG9CQUFvQixHQUFHLDBCQUEwQixDQUFDO0FBQ3hELElBQU0sd0JBQXdCLEdBQUcsMEJBQTBCLENBQUM7QUFFNUQsSUFBTSxXQUFXLEdBQUcsTUFBTSxDQUFDO0FBQzNCLElBQU0saUJBQWlCLEdBQUcsWUFBWSxDQUFDO0FBQ3ZDLElBQU0sbUJBQW1CLEdBQUcsY0FBYyxDQUFDO0FBQzNDLElBQU0scUJBQXFCLEdBQUcsZ0JBQWdCLENBQUM7QUFTL0M7SUFBMEIsZ0NBQXVCO0lBQWpEOztJQVNBLENBQUM7SUFSVSxvQkFBSyxHQUFaLFVBQWEsVUFBeUI7UUFDbEMsT0FBTyxJQUFJLENBQUMsZUFBZSxDQUFDLElBQUksY0FBYyxDQUFDLFVBQVUsRUFBRSxJQUFJLENBQUMsUUFBUSxFQUFFO1lBQ3RFLElBQUksRUFBRSxJQUFJLENBQUMsYUFBYSxDQUFDLE9BQU8sQ0FBQyxXQUFXLENBQUMsS0FBSyxDQUFDLENBQUM7WUFDcEQsVUFBVSxFQUFFLElBQUksQ0FBQyxhQUFhLENBQUMsT0FBTyxDQUFDLGlCQUFpQixDQUFDLEtBQUssQ0FBQyxDQUFDO1lBQ2hFLEtBQUssRUFBRSxJQUFJLENBQUMsYUFBYSxDQUFDLE9BQU8sQ0FBQyxtQkFBbUIsQ0FBQyxLQUFLLENBQUMsQ0FBQztZQUM3RCxZQUFZLEVBQUUsSUFBSSxDQUFDLGFBQWEsQ0FBQyxPQUFPLENBQUMscUJBQXFCLENBQUMsS0FBSyxDQUFDLENBQUM7U0FDekUsQ0FBQyxDQUFDLENBQUM7SUFDUixDQUFDO0lBQ0wsV0FBQztBQUFELENBQUMsQUFURCxDQUEwQixJQUFJLENBQUMsS0FBSyxDQUFDLFlBQVksR0FTaEQ7QUFUWSxvQkFBSTtBQVdq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