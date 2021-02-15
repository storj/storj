"use strict";
Object.defineProperty(exports, "__esModule", { value: true });
var tslib_1 = require("tslib");
var Lint = require("tslint");
var AbstractConfigDependentRule = (function (_super) {
    tslib_1.__extends(AbstractConfigDependentRule, _super);
    function AbstractConfigDependentRule() {
        return _super !== null && _super.apply(this, arguments) || this;
    }
    AbstractConfigDependentRule.prototype.isEnabled = function () {
        return _super.prototype.isEnabled.call(this) && this.ruleArguments.length !== 0;
    };
    return AbstractConfigDependentRule;
}(Lint.Rules.AbstractRule));
exports.AbstractConfigDependentRule = AbstractConfigDependentRule;
//# sourceMappingURL=data:application/json;base64,eyJ2ZXJzaW9uIjozLCJmaWxlIjoicnVsZXMuanMiLCJzb3VyY2VSb290IjoiIiwic291cmNlcyI6WyJydWxlcy50cyJdLCJuYW1lcyI6W10sIm1hcHBpbmdzIjoiOzs7QUFBQSw2QkFBK0I7QUFFL0I7SUFBMEQsdURBQXVCO0lBQWpGOztJQUlBLENBQUM7SUFIVSwrQ0FBUyxHQUFoQjtRQUNJLE9BQU8saUJBQU0sU0FBUyxXQUFFLElBQUksSUFBSSxDQUFDLGFBQWEsQ0FBQyxNQUFNLEtBQUssQ0FBQyxDQUFDO0lBQ2hFLENBQUM7SUFDTCxrQ0FBQztBQUFELENBQUMsQUFKRCxDQUEwRCxJQUFJLENBQUMsS0FBSyxDQUFDLFlBQVksR0FJaEY7QUFKcUIsa0VBQTJCIn0=