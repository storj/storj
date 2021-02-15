"use strict";
Object.defineProperty(exports, "__esModule", { value: true });
var tslib_1 = require("tslib");
var ts = require("typescript");
var Lint = require("tslint");
var utils = require("tsutils");
var rules_1 = require("../src/rules");
var ALL_OR_NONE_OPTION = 'all-or-none';
var LEADING_OPTION = 'leading';
var TRAILING_OPTION = 'trailing';
var READONLY_OPTION = 'readonly';
var MEMBER_ACCESS_OPTION = 'member-access';
var ALL_OR_NONE_FAIL = 'don\'t mix parameter properties with regular parameters';
var LEADING_FAIL = 'parameter properties must precede regular parameters';
var TRAILING_FAIL = 'regular parameters must precede parameter properties';
var READONLY_FAIL = 'parameter property must be readonly';
var MEMBER_ACCESS_FAIL = 'parameter property must have access modifier';
var Rule = (function (_super) {
    tslib_1.__extends(Rule, _super);
    function Rule() {
        return _super !== null && _super.apply(this, arguments) || this;
    }
    Rule.prototype.apply = function (sourceFile) {
        return this.applyWithWalker(new ParameterPropertyWalker(sourceFile, this.ruleName, {
            allOrNone: this.ruleArguments.indexOf(ALL_OR_NONE_OPTION) !== -1,
            leading: this.ruleArguments.indexOf(LEADING_OPTION) !== -1,
            trailing: this.ruleArguments.indexOf(TRAILING_OPTION) !== -1,
            readOnly: this.ruleArguments.indexOf(READONLY_OPTION) !== -1,
            memberAccess: this.ruleArguments.indexOf(MEMBER_ACCESS_OPTION) !== -1,
        }));
    };
    return Rule;
}(rules_1.AbstractConfigDependentRule));
exports.Rule = Rule;
var ParameterPropertyWalker = (function (_super) {
    tslib_1.__extends(ParameterPropertyWalker, _super);
    function ParameterPropertyWalker() {
        return _super !== null && _super.apply(this, arguments) || this;
    }
    ParameterPropertyWalker.prototype.walk = function (sourceFile) {
        var _this = this;
        var cb = function (node) {
            if (node.kind === ts.SyntaxKind.Constructor)
                _this._checkConstructorDeclaration(node);
            return ts.forEachChild(node, cb);
        };
        return ts.forEachChild(sourceFile, cb);
    };
    ParameterPropertyWalker.prototype._checkConstructorDeclaration = function (node) {
        var parameters = node.parameters;
        var length = parameters.length;
        if (length === 0)
            return;
        var index = -1;
        for (var i = 0; i < length; ++i) {
            if (utils.isParameterProperty(parameters[i])) {
                index = i;
                break;
            }
        }
        if (index === -1)
            return;
        if (this.options.allOrNone) {
            var start = parameters[0].getStart(this.getSourceFile());
            var end = parameters[parameters.length - 1].getEnd();
            if (index > 0) {
                this.addFailure(start, end, ALL_OR_NONE_FAIL);
            }
            else {
                for (var i = index + 1; i < length; ++i) {
                    if (!utils.isParameterProperty(parameters[i])) {
                        this.addFailure(start, end, ALL_OR_NONE_FAIL);
                        break;
                    }
                }
            }
        }
        else if (this.options.leading) {
            var regular = index > 0;
            for (var i = index; i < length; ++i) {
                if (utils.isParameterProperty(parameters[i])) {
                    if (regular)
                        this.addFailureAtNode(parameters[i], LEADING_FAIL);
                }
                else {
                    regular = true;
                }
            }
        }
        else if (this.options.trailing) {
            for (var i = index; i < length; ++i)
                if (!utils.isParameterProperty(parameters[i]))
                    this.addFailureAtNode(parameters[i], TRAILING_FAIL);
        }
        if (this.options.memberAccess) {
            for (var i = index; i < length; ++i) {
                var parameter = parameters[i];
                if (utils.isParameterProperty(parameter) && !utils.hasAccessModifier(parameter))
                    this.addFailureAtNode(parameter, MEMBER_ACCESS_FAIL);
            }
        }
        if (this.options.readOnly) {
            for (var i = index; i < length; ++i) {
                var parameter = parameters[i];
                if (utils.isParameterProperty(parameter) && !utils.hasModifier(parameter.modifiers, ts.SyntaxKind.ReadonlyKeyword))
                    this.addFailureAtNode(parameter, READONLY_FAIL);
            }
        }
    };
    return ParameterPropertyWalker;
}(Lint.AbstractWalker));
//# sourceMappingURL=data:application/json;base64,eyJ2ZXJzaW9uIjozLCJmaWxlIjoicGFyYW1ldGVyUHJvcGVydGllc1J1bGUuanMiLCJzb3VyY2VSb290IjoiIiwic291cmNlcyI6WyJwYXJhbWV0ZXJQcm9wZXJ0aWVzUnVsZS50cyJdLCJuYW1lcyI6W10sIm1hcHBpbmdzIjoiOzs7QUFBQSwrQkFBaUM7QUFDakMsNkJBQStCO0FBQy9CLCtCQUFpQztBQUVqQyxzQ0FBMkQ7QUFFM0QsSUFBTSxrQkFBa0IsR0FBRyxhQUFhLENBQUM7QUFDekMsSUFBTSxjQUFjLEdBQUcsU0FBUyxDQUFDO0FBQ2pDLElBQU0sZUFBZSxHQUFHLFVBQVUsQ0FBQztBQUNuQyxJQUFNLGVBQWUsR0FBRyxVQUFVLENBQUM7QUFDbkMsSUFBTSxvQkFBb0IsR0FBRyxlQUFlLENBQUM7QUFFN0MsSUFBTSxnQkFBZ0IsR0FBRyx5REFBeUQsQ0FBQztBQUNuRixJQUFNLFlBQVksR0FBRyxzREFBc0QsQ0FBQztBQUM1RSxJQUFNLGFBQWEsR0FBRyxzREFBc0QsQ0FBQztBQUM3RSxJQUFNLGFBQWEsR0FBRyxxQ0FBcUMsQ0FBQztBQUM1RCxJQUFNLGtCQUFrQixHQUFHLDhDQUE4QyxDQUFDO0FBbUIxRTtJQUEwQixnQ0FBMkI7SUFBckQ7O0lBVUEsQ0FBQztJQVRVLG9CQUFLLEdBQVosVUFBYSxVQUF5QjtRQUNsQyxPQUFPLElBQUksQ0FBQyxlQUFlLENBQUMsSUFBSSx1QkFBdUIsQ0FBQyxVQUFVLEVBQUUsSUFBSSxDQUFDLFFBQVEsRUFBRTtZQUMvRSxTQUFTLEVBQUUsSUFBSSxDQUFDLGFBQWEsQ0FBQyxPQUFPLENBQUMsa0JBQWtCLENBQUMsS0FBSyxDQUFDLENBQUM7WUFDaEUsT0FBTyxFQUFFLElBQUksQ0FBQyxhQUFhLENBQUMsT0FBTyxDQUFDLGNBQWMsQ0FBQyxLQUFLLENBQUMsQ0FBQztZQUMxRCxRQUFRLEVBQUUsSUFBSSxDQUFDLGFBQWEsQ0FBQyxPQUFPLENBQUMsZUFBZSxDQUFDLEtBQUssQ0FBQyxDQUFDO1lBQzVELFFBQVEsRUFBRSxJQUFJLENBQUMsYUFBYSxD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