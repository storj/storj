"use strict";
Object.defineProperty(exports, "__esModule", { value: true });
const ymir_1 = require("@fimbul/ymir");
const TSLint = require("tslint");
const ts = require("typescript");
const getCaller = require("get-caller-file");
const path = require("path");
const tsutils_1 = require("tsutils");
const tslib_1 = require("tslib");
// tslint:disable-next-line:naming-convention
function wrapTslintRule(Rule, name = inferName(Rule)) {
    class R extends ymir_1.AbstractRule {
        constructor(context) {
            super(context);
            this.delegate = new Rule({
                ruleArguments: TSLint.Utils.arrayify(context.options),
                ruleSeverity: 'error',
                ruleName: name,
                disabledIntervals: [],
            });
        }
        apply() {
            if (!this.delegate.isEnabled())
                return;
            let result;
            if (TSLint.isTypedRule(this.delegate) && this.program !== undefined) {
                result = this.delegate.applyWithProgram(this.sourceFile, this.program);
            }
            else {
                result = this.delegate.apply(this.sourceFile);
            }
            const { fileName } = this.sourceFile;
            for (const failure of result) {
                if (failure.getFileName() !== fileName)
                    throw new Error(`Adding findings for a different SourceFile is not supported. Expected '${fileName}' but received '${failure.getFileName()}' from rule '${this.delegate.getOptions().ruleName}'.`);
                this.addFinding(failure.getStartPosition().getPosition(), failure.getEndPosition().getPosition(), failure.getFailure(), arrayify(failure.getFix()).map((r) => ({ start: r.start, end: r.end, text: r.text })));
            }
        }
    }
    R.requiresTypeInformation = !!(Rule.metadata && Rule.metadata.requiresTypeInfo) ||
        Rule.prototype instanceof TSLint.Rules.TypedRule;
    R.deprecated = Rule.metadata && typeof Rule.metadata.deprecationMessage === 'string'
        ? Rule.metadata.deprecationMessage || true // empty deprecation message is coerced to true
        : false;
    return Rule.metadata && Rule.metadata.typescriptOnly ? tslib_1.__decorate([ymir_1.typescriptOnly], R) : R;
}
exports.wrapTslintRule = wrapTslintRule;
function inferName(Rule) {
    if (Rule.metadata !== undefined && Rule.metadata.ruleName)
        return Rule.metadata.ruleName;
    const caller = getCaller(3);
    return path.basename(caller, path.extname(caller));
}
function wrapTslintFormatter(Formatter) {
    return class extends ymir_1.AbstractFormatter {
        constructor() {
            super();
            this.fileNames = [];
            this.failures = [];
            this.fixed = [];
            this.delegate = new Formatter();
        }
        format(fileName, summary) {
            this.fileNames.push(fileName);
            let sourceFile;
            for (let i = 0; i < summary.fixes; ++i)
                this.fixed.push(new TSLint.RuleFailure(getSourceFile(), 0, 0, '', '', TSLint.Replacement.appendText(0, '')));
            if (summary.findings.length === 0)
                return;
            this.failures.push(...summary.findings.map((f) => {
                const failure = new TSLint.RuleFailure(getSourceFile(), f.start.position, f.end.position, f.message, f.ruleName, f.fix && f.fix.replacements.map(convertToTslintReplacement));
                failure.setRuleSeverity(f.severity === 'suggestion' ? 'warning' : f.severity);
                return failure;
            }));
            return;
            function getSourceFile() {
                return sourceFile ||
                    (sourceFile = ts.createSourceFile(fileName, summary.content, ts.ScriptTarget.Latest));
            }
        }
        flush() {
            return this.delegate.format(this.failures, this.fixed, this.fileNames).trim();
        }
    };
}
exports.wrapTslintFormatter = wrapTslintFormatter;
// tslint:disable-next-line:naming-convention
function wrapRuleForTslint(Rule) {
    var _a, _b;
    const metadata = {
        ruleName: 'who-cares',
        typescriptOnly: false,
        description: '',
        options: undefined,
        optionsDescription: '',
        type: 'functionality',
        deprecationMessage: !Rule.deprecated ? undefined : Rule.deprecated === true ? '' : Rule.deprecated,
    };
    function apply(options, sourceFile, program) {
        const args = options.ruleArguments.length < 2 ? options.ruleArguments[0] : options.ruleArguments;
        const failures = [];
        const context = {
            sourceFile,
            program,
            compilerOptions: program && program.getCompilerOptions(),
            options: args,
            settings: new Map(),
            getFlatAst() {
                return tsutils_1.convertAst(sourceFile).flat;
            },
            getWrappedAst() {
                return tsutils_1.convertAst(sourceFile).wrapped;
            },
            addFinding(start, end, message, fix) {
                failures.push(new TSLint.RuleFailure(sourceFile, start, end, message, options.ruleName, fix && arrayify(fix).map(convertToTslintReplacement)));
            },
        };
        if (Rule.supports === undefined || Rule.supports(sourceFile, context) === true)
            new Rule(context).apply();
        return failures;
    }
    if (Rule.requiresTypeInformation)
        return _a = class extends TSLint.Rules.TypedRule {
                applyWithProgram(sourceFile, program) {
                    return apply(this.getOptions(), sourceFile, program);
                }
            },
            _a.metadata = metadata,
            _a;
    return _b = class extends TSLint.Rules.OptionallyTypedRule {
            apply(sourceFile) {
                return apply(this.getOptions(), sourceFile);
            }
            applyWithProgram(sourceFile, program) {
                return apply(this.getOptions(), sourceFile, program);
            }
        },
        _b.metadata = metadata,
        _b;
}
exports.wrapRuleForTslint = wrapRuleForTslint;
function convertToTslintReplacement(r) {
    return new TSLint.Replacement(r.start, r.end - r.start, r.text);
}
function arrayify(maybeArr) {
    return Array.isArray(maybeArr)
        ? maybeArr
        : maybeArr === undefined
            ? []
            : [maybeArr];
}
//# sourceMappingURL=index.js.map