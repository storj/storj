"use strict";
Object.defineProperty(exports, "__esModule", { value: true });
require("reflect-metadata");
const ts = require("typescript");
const tsutils_1 = require("tsutils");
const path = require("path");
class ConfigurationError extends Error {
}
exports.ConfigurationError = ConfigurationError;
class GlobalOptions {
}
exports.GlobalOptions = GlobalOptions;
exports.Replacement = {
    replace(start, end, text) {
        return { start, end, text };
    },
    append(pos, text) {
        return { start: pos, end: pos, text }; // tslint:disable-line:object-shorthand-properties-first
    },
    delete(start, end) {
        return { start, end, text: '' };
    },
};
exports.Finding = {
    /** Compare two Findings. Intended to be used in `Array.prototype.sort`. */
    compare(a, b) {
        return a.start.position - b.start.position
            || a.end.position - b.end.position
            || compareStrings(a.ruleName, b.ruleName)
            || compareStrings(a.message, b.message);
    },
};
function compareStrings(a, b) {
    return a < b
        ? -1
        : a > b
            ? 1
            : 0;
}
function predicate(check) {
    return (target) => {
        target.supports = combinePredicates(target.supports, check);
    };
}
exports.predicate = predicate;
function combinePredicates(existing, additonal) {
    if (existing === undefined)
        return additonal;
    return (sourceFile, context) => {
        const result = additonal(sourceFile, context);
        return result !== true ? result : existing(sourceFile, context);
    };
}
function typescriptOnly(target) {
    target.supports = combinePredicates(target.supports, (sourceFile) => /\.tsx?$/.test(sourceFile.fileName) || 'TypeScript only');
}
exports.typescriptOnly = typescriptOnly;
function excludeDeclarationFiles(target) {
    target.supports = combinePredicates(target.supports, (sourceFile) => !sourceFile.isDeclarationFile || 'excludes declaration files');
}
exports.excludeDeclarationFiles = excludeDeclarationFiles;
function requireLibraryFile(fileName) {
    return (target) => {
        target.supports = combinePredicates(target.supports, (_, context) => programContainsLibraryFile(context.program, fileName) || `requires library file '${fileName}'`);
    };
}
exports.requireLibraryFile = requireLibraryFile;
function programContainsLibraryFile(program, fileName) {
    const libFileDir = path.dirname(ts.getDefaultLibFilePath(program.getCompilerOptions()));
    return program.getSourceFile(path.join(libFileDir, fileName)) !== undefined;
}
function requiresCompilerOption(option) {
    return (target) => {
        target.supports = combinePredicates(target.supports, (_, context) => tsutils_1.isCompilerOptionEnabled(context.compilerOptions, option) || `requires compilerOption '${option}'`);
    };
}
exports.requiresCompilerOption = requiresCompilerOption;
class AbstractRule {
    constructor(context) {
        this.context = context;
        this.sourceFile = context.sourceFile;
    }
    get program() {
        return this.context.program;
    }
    addFinding(start, end, message, fix) {
        return this.context.addFinding(start, end, message, fix);
    }
    addFindingAtNode(node, message, fix) {
        return this.addFinding(node.getStart(this.sourceFile), node.end, message, fix);
    }
}
AbstractRule.requiresTypeInformation = false;
AbstractRule.deprecated = false;
AbstractRule.supports = undefined;
exports.AbstractRule = AbstractRule;
class ConfigurableRule extends AbstractRule {
    constructor(context) {
        super(context);
        this.options = this.parseOptions(context.options);
    }
}
exports.ConfigurableRule = ConfigurableRule;
class TypedRule extends AbstractRule {
    constructor(context) {
        super(context);
    }
    /** Lazily evaluated getter for TypeChecker. Use this instead of `this.program.getTypeChecker()` to avoid wasting CPU cycles. */
    get checker() {
        const checker = this.program.getTypeChecker();
        Object.defineProperty(this, 'checker', { value: checker, writable: false });
        return checker;
    }
}
TypedRule.requiresTypeInformation = true;
exports.TypedRule = TypedRule;
class ConfigurableTypedRule extends TypedRule {
    constructor(context) {
        super(context);
        this.options = this.parseOptions(context.options);
    }
}
exports.ConfigurableTypedRule = ConfigurableTypedRule;
class AbstractFormatter {
}
exports.AbstractFormatter = AbstractFormatter;
class ConfigurationProvider {
}
exports.ConfigurationProvider = ConfigurationProvider;
var Format;
(function (Format) {
    Format["Yaml"] = "yaml";
    Format["Json"] = "json";
    Format["Json5"] = "json5";
})(Format = exports.Format || (exports.Format = {}));
class AbstractProcessor {
    /**
     * Returns a new primary extension that is appended to the file name, e.g. '.ts'.
     * If the file should not get a new extension, just return an empty string.
     */
    static getSuffixForFile(_context) {
        return '';
    }
    constructor(context) {
        this.source = context.source;
        this.sourceFileName = context.sourceFileName;
        this.targetFileName = context.targetFileName;
        this.settings = context.settings;
    }
}
exports.AbstractProcessor = AbstractProcessor;
class MessageHandler {
}
exports.MessageHandler = MessageHandler;
class DeprecationHandler {
}
exports.DeprecationHandler = DeprecationHandler;
var DeprecationTarget;
(function (DeprecationTarget) {
    DeprecationTarget["Rule"] = "rule";
    DeprecationTarget["Processor"] = "processor";
    DeprecationTarget["Formatter"] = "formatter";
})(DeprecationTarget = exports.DeprecationTarget || (exports.DeprecationTarget = {}));
class FileSystem {
}
exports.FileSystem = FileSystem;
class RuleLoaderHost {
}
exports.RuleLoaderHost = RuleLoaderHost;
class FormatterLoaderHost {
}
exports.FormatterLoaderHost = FormatterLoaderHost;
// wotan-enable no-misused-generics
class CacheFactory {
}
exports.CacheFactory = CacheFactory;
class Resolver {
}
exports.Resolver = Resolver;
class BuiltinResolver {
}
exports.BuiltinResolver = BuiltinResolver;
class DirectoryService {
}
exports.DirectoryService = DirectoryService;
class FindingFilterFactory {
}
exports.FindingFilterFactory = FindingFilterFactory;
class LineSwitchParser {
}
exports.LineSwitchParser = LineSwitchParser;
class FileFilterFactory {
}
exports.FileFilterFactory = FileFilterFactory;
//# sourceMappingURL=index.js.map