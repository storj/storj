"use strict";
Object.defineProperty(exports, "__esModule", { value: true });
var tslib_1 = require("tslib");
var ts = require("typescript");
var Lint = require("tslint");
var tsutils_1 = require("tsutils");
var CHECK_RETURN_TYPE_OPTION = 'check-return-type';
var FAIL_MESSAGE = "type annotation is redundant";
var Rule = (function (_super) {
    tslib_1.__extends(Rule, _super);
    function Rule() {
        return _super !== null && _super.apply(this, arguments) || this;
    }
    Rule.prototype.applyWithProgram = function (sourceFile, program) {
        return this.applyWithFunction(sourceFile, walk, {
            checkReturnType: this.ruleArguments.indexOf(CHECK_RETURN_TYPE_OPTION) !== -1,
        }, program.getTypeChecker());
    };
    return Rule;
}(Lint.Rules.TypedRule));
exports.Rule = Rule;
var formatFlags = ts.TypeFormatFlags.UseStructuralFallback
    | ts.TypeFormatFlags.UseFullyQualifiedType
    | ts.TypeFormatFlags.UseAliasDefinedOutsideCurrentScope
    | ts.TypeFormatFlags.NoTruncation
    | ts.TypeFormatFlags.WriteClassExpressionAsTypeLiteral
    | ts.TypeFormatFlags.WriteArrowStyleSignature;
function walk(ctx, checker) {
    return ts.forEachChild(ctx.sourceFile, function cb(node) {
        switch (node.kind) {
            case ts.SyntaxKind.ArrowFunction:
            case ts.SyntaxKind.FunctionExpression:
                checkFunction(node);
                break;
            case ts.SyntaxKind.MethodDeclaration:
                if (node.parent.kind === ts.SyntaxKind.ObjectLiteralExpression)
                    checkObjectLiteralMethod(node);
                break;
            case ts.SyntaxKind.VariableDeclarationList:
                checkVariables(node);
        }
        return ts.forEachChild(node, cb);
    });
    function checkFunction(node) {
        if (!functionHasTypeDeclarations(node))
            return;
        var iife = tsutils_1.getIIFE(node);
        if (iife !== undefined)
            return checkIife(node, iife);
        var type = getContextualTypeOfFunction(node);
        if (type === undefined)
            return;
        checkContextSensitiveFunctionOrMethod(node, type);
    }
    function checkObjectLiteralMethod(node) {
        if (!functionHasTypeDeclarations(node))
            return;
        var type = getContextualTypeOfObjectLiteralMethod(node);
        if (type === undefined)
            return;
        checkContextSensitiveFunctionOrMethod(node, type);
    }
    function checkContextSensitiveFunctionOrMethod(node, contextualType) {
        var parameters = parametersExceptThis(node.parameters);
        var sig = getMatchingSignature(contextualType, parameters);
        if (sig === undefined)
            return;
        var signature = sig[0], checkReturn = sig[1];
        if (ctx.options.checkReturnType && checkReturn && node.type !== undefined && !signatureHasGenericOrTypePredicateReturn(signature) &&
            typesAreEqual(checker.getTypeFromTypeNode(node.type), signature.getReturnType()))
            fail(node.type);
        var restParameterContext = false;
        var contextualParameterType;
        for (var i = 0; i < parameters.length; ++i) {
            if (!restParameterContext) {
                var context = signature.parameters[i];
                if (context === undefined || context.valueDeclaration === undefined)
                    break;
                if (tsutils_1.isTypeParameter(checker.getTypeAtLocation(context.valueDeclaration)))
                    continue;
                contextualParameterType = checker.getTypeOfSymbolAtLocation(context, node);
                if (context.valueDeclaration.dotDotDotToken !== undefined) {
                    var indexType = contextualParameterType.getNumberIndexType();
                    if (indexType === undefined)
                        break;
                    contextualParameterType = indexType;
                    restParameterContext = true;
                }
            }
            var parameter = parameters[i];
            if (parameter.type === undefined)
                continue;
            var declaredType = void 0;
            if (parameter.dotDotDotToken !== undefined) {
                if (!restParameterContext)
                    break;
                declaredType = checker.getTypeFromTypeNode(parameter.type);
                var indexType = declaredType.getNumberIndexType();
                if (indexType === undefined)
                    break;
                declaredType = indexType;
            }
            else {
                declaredType = checker.getTypeFromTypeNode(parameter.type);
            }
            if (compareParameterTypes(contextualParameterType, declaredType, parameter.questionToken !== undefined || parameter.initializer !== undefined))
                fail(parameter.type);
        }
    }
    function checkIife(func, iife) {
        if (ctx.options.checkReturnType && func.type !== undefined && func.name === undefined &&
            (!tsutils_1.isExpressionValueUsed(iife) ||
                !containsTypeWithFlag(checker.getTypeFromTypeNode(func.type), ts.TypeFlags.Literal) &&
                    checker.getContextualType(iife) !== undefined))
            fail(func.type);
        var parameters = parametersExceptThis(func.parameters);
        var args = iife.arguments;
        var len = Math.min(parameters.length, args.length);
        outer: for (var i = 0; i < len; ++i) {
            var parameter = parameters[i];
            if (parameter.type === undefined)
                continue;
            var declaredType = checker.getTypeFromTypeNode(parameter.type);
            var contextualType = checker.getBaseTypeOfLiteralType(checker.getTypeAtLocation(args[i]));
            if (parameter.dotDotDotToken !== undefined) {
                var indexType = declaredType.getNumberIndexType();
                if (indexType === undefined || !typesAreEqual(indexType, contextualType))
                    break;
                for (var j = i + 1; j < args.length; ++j)
                    if (!typesAreEqual(contextualType, checker.getBaseTypeOfLiteralType(checker.getTypeAtLocation(args[j]))))
                        break outer;
                fail(parameter.type);
            }
            else if (compareParameterTypes(contextualType, declaredType, parameter.questionToken !== undefined || parameter.initializer !== undefined)) {
                fail(parameter.type);
            }
        }
    }
    function checkVariables(list) {
        var isConst = tsutils_1.getVariableDeclarationKind(list) === 2;
        for (var _i = 0, _a = list.declarations; _i < _a.length; _i++) {
            var variable = _a[_i];
            if (variable.type === undefined || variable.initializer === undefined)
                continue;
            var inferred = checker.getTypeAtLocation(variable.initializer);
            if (!isConst)
                inferred = checker.getBaseTypeOfLiteralType(inferred);
            var declared = checker.getTypeFromTypeNode(variable.type);
            if (typesAreEqual(declared, inferred) || isConst && typesAreEqual(declared, checker.getBaseTypeOfLiteralType(inferred)))
                fail(variable.type);
        }
    }
    function fail(type) {
        ctx.addFailure(type.pos - 1, type.end, FAIL_MESSAGE, Lint.Replacement.deleteFromTo(type.pos - 1, type.end));
    }
    function typesAreEqual(a, b) {
        return a === b || checker.typeToString(a, undefined, formatFlags) === checker.typeToString(b, undefined, formatFlags);
    }
    function getContextualTypeOfFunction(func) {
        var type = checker.getContextualType(func);
        return type && checker.getApparentType(type);
    }
    function getContextualTypeOfObjectLiteralMethod(method) {
        var type = checker.getContextualType(method.parent);
        if (type === undefined)
            return;
        type = checker.getApparentType(type);
        if (!tsutils_1.isTypeFlagSet(type, ts.TypeFlags.StructuredType))
            return;
        var t = checker.getTypeAtLocation(method);
        var symbol = t.symbol && type.getProperties().find(function (s) { return s.escapedName === t.symbol.escapedName; });
        return symbol !== undefined
            ? checker.getTypeOfSymbolAtLocation(symbol, method.name)
            : isNumericPropertyName(method.name) && type.getNumberIndexType() || type.getStringIndexType();
    }
    function signatureHasGenericOrTypePredicateReturn(signature) {
        if (signature.declaration === undefined)
            return false;
        if (signature.declaration.type !== undefined && tsutils_1.isTypePredicateNode(signature.declaration.type))
            return true;
        var original = checker.getSignatureFromDeclaration(signature.declaration);
        return original !== undefined && tsutils_1.isTypeParameter(original.getReturnType());
    }
    function removeOptionalityFromType(type) {
        if (!containsTypeWithFlag(type, ts.TypeFlags.Undefined))
            return type;
        var allowsNull = containsTypeWithFlag(type, ts.TypeFlags.Null);
        type = checker.getNonNullableType(type);
        return allowsNull ? checker.getNullableType(type, ts.TypeFlags.Null) : type;
    }
    function compareParameterTypes(context, declared, optional) {
        if (optional)
            declared = removeOptionalityFromType(declared);
        return typesAreEqual(declared, context) ||
            optional && typesAreEqual(checker.getNullableType(declared, ts.TypeFlags.Undefined), context);
    }
    function isNumericPropertyName(name) {
        var str = tsutils_1.getPropertyName(name);
        if (str !== undefined)
            return tsutils_1.isValidNumericLiteral(str) && String(+str) === str;
        return isAssignableToNumber(checker.getTypeAtLocation(name.expression));
    }
    function isAssignableToNumber(type) {
        var typeParametersSeen;
        return (function check(t) {
            if (tsutils_1.isTypeParameter(t) && t.symbol !== undefined && t.symbol.declarations !== undefined) {
                if (typeParametersSeen === undefined) {
                    typeParametersSeen = new Set([t]);
                }
                else if (!typeParametersSeen.has(t)) {
                    typeParametersSeen.add(t);
                }
                else {
                    return false;
                }
                var declaration = t.symbol.declarations[0];
                if (declaration.constraint === undefined)
                    return true;
                return check(checker.getTypeFromTypeNode(declaration.constraint));
            }
            if (tsutils_1.isUnionType(t))
                return t.types.every(check);
            if (tsutils_1.isIntersectionType(t))
                return t.types.some(check);
            return tsutils_1.isTypeFlagSet(t, ts.TypeFlags.NumberLike | ts.TypeFlags.Any);
        })(type);
    }
    function getMatchingSignature(type, parameters) {
        var minArguments = getMinArguments(parameters);
        var signatures = getSignaturesOfType(type).filter(function (s) { return s.declaration !== undefined &&
            getNumParameters(s.declaration.parameters) >= minArguments; });
        switch (signatures.length) {
            case 0:
                return;
            case 1:
                return [signatures[0], true];
            default: {
                var str = checker.signatureToString(signatures[0], undefined, formatFlags);
                var withoutReturn = removeSignatureReturn(str);
                var returnUsable = true;
                for (var i = 1; i < signatures.length; ++i) {
                    var sig = checker.signatureToString(signatures[i], undefined, formatFlags);
                    if (str !== sig) {
                        if (withoutReturn !== removeSignatureReturn(sig))
                            return;
                        returnUsable = false;
                    }
                }
                return [signatures[0], returnUsable];
            }
        }
    }
}
function removeSignatureReturn(str) {
    var sourceFile = ts.createSourceFile('tmp.ts', "type T=" + str, ts.ScriptTarget.ESNext);
    var signature = sourceFile.statements[0].type;
    return sourceFile.text.substring(7, signature.parameters.end + 1);
}
function getSignaturesOfType(type) {
    if (tsutils_1.isUnionType(type)) {
        var signatures = [];
        for (var _i = 0, _a = type.types; _i < _a.length; _i++) {
            var t = _a[_i];
            signatures.push.apply(signatures, getSignaturesOfType(t));
        }
        return signatures;
    }
    if (tsutils_1.isIntersectionType(type)) {
        var signatures = void 0;
        for (var _b = 0, _c = type.types; _b < _c.length; _b++) {
            var t = _c[_b];
            var sig = getSignaturesOfType(t);
            if (sig.length !== 0) {
                if (signatures !== undefined)
                    return [];
                signatures = sig;
            }
        }
        return signatures === undefined ? [] : signatures;
    }
    return type.getCallSignatures();
}
function getNumParameters(parameters) {
    if (parameters.length === 0)
        return 0;
    if (parameters[parameters.length - 1].dotDotDotToken !== undefined)
        return Infinity;
    return parametersExceptThis(parameters).length;
}
function getMinArguments(parameters) {
    var minArguments = parameters.length;
    for (; minArguments > 0; --minArguments) {
        var parameter = parameters[minArguments - 1];
        if (parameter.questionToken === undefined && parameter.initializer === undefined && parameter.dotDotDotToken === undefined)
            break;
    }
    return minArguments;
}
function containsTypeWithFlag(type, flag) {
    return tsutils_1.isUnionType(type) ? type.types.some(function (t) { return tsutils_1.isTypeFlagSet(t, flag); }) : tsutils_1.isTypeFlagSet(type, flag);
}
function parametersExceptThis(parameters) {
    return parameters.length !== 0 && tsutils_1.isThisParameter(parameters[0]) ? parameters.slice(1) : parameters;
}
function functionHasTypeDeclarations(func) {
    return func.type !== undefined || parametersExceptThis(func.parameters).some(function (p) { return p.type !== undefined; });
}
//# sourceMappingURL=data:application/json;base64,eyJ2ZXJzaW9uIjozLCJmaWxlIjoibm9Vbm5lY2Vzc2FyeVR5cGVBbm5vdGF0aW9uUnVsZS5qcyIsInNvdXJjZVJvb3QiOiIiLCJzb3VyY2VzIjpbIm5vVW5uZWNlc3NhcnlUeXBlQW5ub3RhdGlvblJ1bGUudHMiXSwibmFtZXMiOltdLCJtYXBwaW5ncyI6Ijs7O0FBQUEsK0JBQWlDO0FBQ2pDLDZCQUErQjtBQUMvQixtQ0FhaUI7QUFJakIsSUFBTSx3QkFBd0IsR0FBRyxtQkFBbUIsQ0FBQztBQUNyRCxJQUFNLFlBQVksR0FBRyw4QkFBOEIsQ0FBQztBQU1wRDtJQUEwQixnQ0FBb0I7SUFBOUM7O0lBVUEsQ0FBQztJQVRVLCtCQUFnQixHQUF2QixVQUF3QixVQUF5QixFQUFFLE9BQW1CO1FBQ2xFLE9BQU8sSUFBSSxDQUFDLGlCQUFpQixDQUN6QixVQUFVLEVBQ1YsSUFBSSxFQUFFO1lBQ0YsZUFBZSxFQUFFLElBQUksQ0FBQyxhQUFhLENBQUMsT0FBTyxDQUFDLHdCQUF3QixDQUFDLEtBQUssQ0FBQyxDQUFDO1NBQy9FLEVBQ0QsT0FBTyxDQUFDLGNBQWMsRUFBRSxDQUMzQixDQUFDO0lBQ04sQ0FBQztJQUNMLFdBQUM7QUFBRCxDQUFDLEFBVkQsQ0FBMEIsSUFBSSxDQUFDLEtBQUssQ0FBQyxTQUFTLEdBVTdDO0FBVlksb0JBQUk7QUFZakIsSUFBTSxXQUFXLEdBQUcsRUFBRSxDQUFDLGVBQWUsQ0FBQyxxQkFBcUI7TUFDdEQsRUFBRSxDQUFDLGVBQWUsQ0FBQyxxQkFBcUI7TUFDeEMsRUFBRSxDQUFDLGVBQWUsQ0FBQyxrQ0FBa0M7TUFDckQsRUFBRSxDQUFDLGVBQWUsQ0FBQyxZQUFZO01BQy9CLEVBQUUsQ0FBQyxlQUFlLENBQUMsaUNBQWlDO01BQ3BELEVBQUUsQ0FBQyxlQUFlLENBQUMsd0JBQXdCLENBQUM7QUFFbEQsU0FBUyxJQUFJLENBQUMsR0FBK0IsRUFBRSxPQUF1QjtJQUNsRSxPQUFPLEVBQUUsQ0FBQyxZQUFZLENBQUMsR0FBRyxDQUFDLFVBQVUsRUFBRSxTQUFTLEVBQUUsQ0FB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