"use strict";
Object.defineProperty(exports, "__esModule", { value: true });
var tslib_1 = require("tslib");
var ts = require("typescript");
var Lint = require("tslint");
var tsutils_1 = require("tsutils");
var AbstractReturnStatementWalker = (function (_super) {
    tslib_1.__extends(AbstractReturnStatementWalker, _super);
    function AbstractReturnStatementWalker() {
        return _super !== null && _super.apply(this, arguments) || this;
    }
    AbstractReturnStatementWalker.prototype.walk = function (sourceFile) {
        var _this = this;
        var cb = function (node) {
            if (node.kind === ts.SyntaxKind.ReturnStatement)
                _this._checkReturnStatement(node);
            return ts.forEachChild(node, cb);
        };
        return ts.forEachChild(sourceFile, cb);
    };
    return AbstractReturnStatementWalker;
}(Lint.AbstractWalker));
exports.AbstractReturnStatementWalker = AbstractReturnStatementWalker;
var AbstractIfStatementWalker = (function (_super) {
    tslib_1.__extends(AbstractIfStatementWalker, _super);
    function AbstractIfStatementWalker() {
        return _super !== null && _super.apply(this, arguments) || this;
    }
    AbstractIfStatementWalker.prototype.walk = function (sourceFile) {
        var _this = this;
        var cb = function (node) {
            if (node.kind === ts.SyntaxKind.IfStatement)
                _this._checkIfStatement(node);
            return ts.forEachChild(node, cb);
        };
        return ts.forEachChild(sourceFile, cb);
    };
    AbstractIfStatementWalker.prototype._reportUnnecessaryElse = function (elseStatement, message) {
        var elseKeyword = elseStatement.parent.getChildAt(5, this.sourceFile);
        if (tsutils_1.isBlock(elseStatement) && !elseStatement.statements.some(isBlockScopedDeclaration)) {
            this.addFailureAtNode(elseKeyword, message, [
                Lint.Replacement.deleteFromTo(elseKeyword.end - 4, elseStatement.statements.pos),
                Lint.Replacement.deleteText(elseStatement.end - 1, 1),
            ]);
        }
        else {
            this.addFailureAtNode(elseKeyword, message, Lint.Replacement.deleteText(elseKeyword.end - 4, 4));
        }
    };
    return AbstractIfStatementWalker;
}(Lint.AbstractWalker));
exports.AbstractIfStatementWalker = AbstractIfStatementWalker;
function isBlockScopedDeclaration(statement) {
    switch (statement.kind) {
        case ts.SyntaxKind.VariableStatement:
            return tsutils_1.isBlockScopedVariableDeclarationList(statement.declarationList);
        case ts.SyntaxKind.ClassDeclaration:
        case ts.SyntaxKind.EnumDeclaration:
        case ts.SyntaxKind.InterfaceDeclaration:
        case ts.SyntaxKind.TypeAliasDeclaration:
            return true;
        default: return false;
    }
}
//# sourceMappingURL=data:application/json;base64,eyJ2ZXJzaW9uIjozLCJmaWxlIjoid2Fsa2VyLmpzIiwic291cmNlUm9vdCI6IiIsInNvdXJjZXMiOlsid2Fsa2VyLnRzIl0sIm5hbWVzIjpbXSwibWFwcGluZ3MiOiI7OztBQUFBLCtCQUFpQztBQUNqQyw2QkFBK0I7QUFDL0IsbUNBQXdFO0FBRXhFO0lBQStELHlEQUFzQjtJQUFyRjs7SUFXQSxDQUFDO0lBVlUsNENBQUksR0FBWCxVQUFZLFVBQXlCO1FBQXJDLGlCQU9DO1FBTkcsSUFBTSxFQUFFLEdBQUcsVUFBQyxJQUFhO1lBQ3JCLElBQUksSUFBSSxDQUFDLElBQUksS0FBSyxFQUFFLENBQUMsVUFBVSxDQUFDLGVBQWU7Z0JBQzNDLEtBQUksQ0FBQyxxQkFBcUIsQ0FBcUIsSUFBSSxDQUFDLENBQUM7WUFDekQsT0FBTyxFQUFFLENBQUMsWUFBWSxDQUFDLElBQUksRUFBRSxFQUFFLENBQUMsQ0FBQztRQUNyQyxDQUFDLENBQUM7UUFDRixPQUFPLEVBQUUsQ0FBQyxZQUFZLENBQUMsVUFBVSxFQUFFLEVBQUUsQ0FBQyxDQUFDO0lBQzNDLENBQUM7SUFHTCxvQ0FBQztBQUFELENBQUMsQUFYRCxDQUErRCxJQUFJLENBQUMsY0FBYyxHQVdqRjtBQVhxQixzRUFBNkI7QUFhbkQ7SUFBMkQscURBQXNCO0lBQWpGOztJQXlCQSxDQUFDO0lBeEJVLHdDQUFJLEdBQVgsVUFBWSxVQUF5QjtRQUFyQyxpQkFPQztRQU5HLElBQU0sRUFBRSxHQUFHLFVBQUMsSUFBYTtZQUNyQixJQUFJLElBQUksQ0FBQyxJQUFJLEtBQUssRUFBRSxDQUFDLFVBQVUsQ0FBQyxXQUFXO2dCQUN2QyxLQUFJLENBQUMsaUJBQWlCLENBQWlCLElBQUksQ0FBQyxDQUFDO1lBQ2pELE9BQU8sRUFBRSxDQUFDLFlBQVksQ0FBQyxJQUFJLEVBQUUsRUFBRSxDQUFDLENBQUM7UUFDckMsQ0FBQyxDQUFDO1FBQ0YsT0FBTyxFQUFFLENBQUMsWUFBWSxDQUFDLFVBQVUsRUFBRSxFQUFFLENBQUMsQ0FBQztJQUMzQyxDQUFDO0lBRVMsMERBQXNCLEdBQWhDLFVBQWlDLGFBQTJCLEVBQUUsT0FBZTtRQUN6RSxJQUFNLFdBQVcsR0FBRyxhQUFhLENBQUMsTUFBTyxDQUFDLFVBQVUsQ0FBQyxDQUFDLEVBQVcsSUFBSSxDQUFDLFVBQVUsQ0FBQyxDQUFDO1FBQ2xGLElBQUksaUJBQU8sQ0FBQyxhQUFhLENBQUMsSUFBSSxDQUFDLGFBQWEsQ0FBQyxVQUFVLENBQUMsSUFBSSxDQUFDLHdCQUF3QixDQUFDLEVBQUU7WUFFcEYsSUFBSSxDQUFDLGdCQUFnQixDQUFDLFdBQVcsRUFBRSxPQUFPLEVBQUU7Z0JBQ3hDLElBQUksQ0FBQyxXQUFXLENBQUMsWUFBWSxDQUFDLFdBQVcsQ0FBQyxHQUFHLEdBQUcsQ0FBQyxFQUFFLGFBQWEsQ0FBQyxVQUFVLENBQUMsR0FBRyxDQUFDO2dCQUNoRixJQUFJLENBQUMsV0FBVyxDQUFDLFVBQVUsQ0FBQyxhQUFhLENBQUMsR0FBRyxHQUFHLENBQUMsRUFBRSxDQUFDLENBQUM7YUFDeEQsQ0FBQyxDQUFDO1NBQ047YUFBTTtZQUVILElBQUksQ0FBQyxnQkFBZ0IsQ0FBQyxXQUFXLEVBQUUsT0FBTyxFQUFFLElBQUksQ0FBQyxXQUFXLENBQUMsVUFBVSxDQUFDLFdBQVcsQ0FBQyxHQUFHLEdBQUcsQ0FBQyxFQUFFLENBQUMsQ0FBQyxDQUFDLENBQUM7U0FDcEc7SUFDTCxDQUFDO0lBR0wsZ0NBQUM7QUFBRCxDQUFDLEFBekJELENBQTJELElBQUksQ0FBQyxjQUFjLEdBeUI3RTtBQXpCcUIsOERBQXlCO0FBNEIvQyxTQUFTLHdCQUF3QixDQUFDLFNBQXVCO0lBQ3JELFFBQVEsU0FBUyxDQUFDLElBQUksRUFBRTtRQUNwQixLQUFLLEVBQUUsQ0FBQyxVQUFVLENBQUMsaUJBQWlCO1lBQ2hDLE9BQU8sOENBQW9DLENBQXdCLFNBQVUsQ0FBQyxlQUFlLENBQUMsQ0FBQztRQUNuRyxLQUFLLEVBQUUsQ0FBQyxVQUFVLENBQUMsZ0JBQWdCLENBQUM7UUFDcEMsS0FBSyxFQUFFLENBQUMsVUFBVSxDQUFDLGVBQWUsQ0FBQztRQUNuQyxLQUFLLEVBQUUsQ0FBQyxVQUFVLENBQUMsb0JBQW9CLENBQUM7UUFDeEMsS0FBSyxFQUFFLENBQUMsVUFBVSxDQUFDLG9CQUFvQjtZQUNuQyxPQUFPLElBQUksQ0FBQztRQUNoQixPQUFPLENBQUMsQ0FBQyxPQUFPLEtBQUssQ0FBQztLQUN6QjtBQUNMLENBQUMifQ==