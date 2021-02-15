import { RuleContext, RuleConstructor, FormatterConstructor } from '@fimbul/ymir';
import * as TSLint from 'tslint';
export declare function wrapTslintRule(Rule: TSLint.RuleConstructor, name?: string): RuleConstructor;
export declare function wrapTslintFormatter(Formatter: TSLint.FormatterConstructor): FormatterConstructor;
export declare function wrapRuleForTslint<T extends RuleContext>(Rule: RuleConstructor<T>): TSLint.RuleConstructor;
