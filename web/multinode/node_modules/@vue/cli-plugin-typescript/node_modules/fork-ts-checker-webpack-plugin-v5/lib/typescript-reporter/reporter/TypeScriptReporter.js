"use strict";
var __awaiter = (this && this.__awaiter) || function (thisArg, _arguments, P, generator) {
    function adopt(value) { return value instanceof P ? value : new P(function (resolve) { resolve(value); }); }
    return new (P || (P = Promise))(function (resolve, reject) {
        function fulfilled(value) { try { step(generator.next(value)); } catch (e) { reject(e); } }
        function rejected(value) { try { step(generator["throw"](value)); } catch (e) { reject(e); } }
        function step(result) { result.done ? resolve(result.value) : adopt(result.value).then(fulfilled, rejected); }
        step((generator = generator.apply(thisArg, _arguments || [])).next());
    });
};
var __importDefault = (this && this.__importDefault) || function (mod) {
    return (mod && mod.__esModule) ? mod : { "default": mod };
};
Object.defineProperty(exports, "__esModule", { value: true });
const path_1 = __importDefault(require("path"));
const TypeScriptIssueFactory_1 = require("../issue/TypeScriptIssueFactory");
const ControlledWatchCompilerHost_1 = require("./ControlledWatchCompilerHost");
const TypeScriptVueExtension_1 = require("../extension/vue/TypeScriptVueExtension");
const ControlledWatchSolutionBuilderHost_1 = require("./ControlledWatchSolutionBuilderHost");
const ControlledTypeScriptSystem_1 = require("./ControlledTypeScriptSystem");
const TypeScriptConfigurationParser_1 = require("./TypeScriptConfigurationParser");
const Performance_1 = require("../../profile/Performance");
const TypeScriptPerformance_1 = require("../profile/TypeScriptPerformance");
function createTypeScriptReporter(configuration) {
    const extensions = [];
    let system;
    let parsedConfiguration;
    let configurationChanged = false;
    let watchCompilerHost;
    let watchSolutionBuilderHost;
    let watchProgram;
    let solutionBuilder;
    const diagnosticsPerProject = new Map();
    // eslint-disable-next-line @typescript-eslint/no-var-requires
    const typescript = require(configuration.typescriptPath);
    const performance = TypeScriptPerformance_1.connectTypeScriptPerformance(typescript, Performance_1.createPerformance());
    if (configuration.extensions.vue.enabled) {
        extensions.push(TypeScriptVueExtension_1.createTypeScriptVueExtension(configuration.extensions.vue));
    }
    function getProjectNameOfBuilderProgram(builderProgram) {
        return builderProgram.getProgram().getCompilerOptions().configFilePath;
    }
    function getDiagnosticsOfBuilderProgram(builderProgram) {
        const diagnostics = [];
        if (configuration.diagnosticOptions.syntactic) {
            performance.markStart('Syntactic Diagnostics');
            diagnostics.push(...builderProgram.getSyntacticDiagnostics());
            performance.markEnd('Syntactic Diagnostics');
        }
        if (configuration.diagnosticOptions.global) {
            performance.markStart('Global Diagnostics');
            diagnostics.push(...builderProgram.getGlobalDiagnostics());
            performance.markEnd('Global Diagnostics');
        }
        if (configuration.diagnosticOptions.semantic) {
            performance.markStart('Semantic Diagnostics');
            diagnostics.push(...builderProgram.getSemanticDiagnostics());
            performance.markEnd('Semantic Diagnostics');
        }
        if (configuration.diagnosticOptions.declaration) {
            performance.markStart('Declaration Diagnostics');
            diagnostics.push(...builderProgram.getDeclarationDiagnostics());
            performance.markEnd('Declaration Diagnostics');
        }
        return diagnostics;
    }
    function emitTsBuildInfoFileForBuilderProgram(builderProgram) {
        if (configuration.mode !== 'readonly' &&
            parsedConfiguration &&
            parsedConfiguration.options.incremental) {
            const program = builderProgram.getProgram();
            // eslint-disable-next-line @typescript-eslint/no-explicit-any
            if (typeof program.emitBuildInfo === 'function') {
                // eslint-disable-next-line @typescript-eslint/no-explicit-any
                program.emitBuildInfo();
            }
        }
    }
    return {
        getReport: ({ changedFiles = [], deletedFiles = [] }) => __awaiter(this, void 0, void 0, function* () {
            if (configuration.profile) {
                performance.enable();
            }
            if (!system) {
                system = ControlledTypeScriptSystem_1.createControlledTypeScriptSystem(typescript, configuration.mode);
            }
            // clear cache to be ready for next iteration and to free memory
            system.clearCache();
            if ([...changedFiles, ...deletedFiles]
                .map((affectedFile) => path_1.default.normalize(affectedFile))
                .includes(path_1.default.normalize(configuration.configFile))) {
                // we need to re-create programs
                parsedConfiguration = undefined;
                watchCompilerHost = undefined;
                watchSolutionBuilderHost = undefined;
                watchProgram = undefined;
                solutionBuilder = undefined;
                diagnosticsPerProject.clear();
                configurationChanged = true;
            }
            if (!parsedConfiguration) {
                const parseConfigurationDiagnostics = [];
                let parseConfigFileHost = Object.assign(Object.assign({}, system), { onUnRecoverableConfigFileDiagnostic: (diagnostic) => {
                        parseConfigurationDiagnostics.push(diagnostic);
                    } });
                extensions.forEach((extension) => {
                    if (extension.extendParseConfigFileHost) {
                        parseConfigFileHost = extension.extendParseConfigFileHost(parseConfigFileHost);
                    }
                });
                performance.markStart('Parse Configuration');
                parsedConfiguration = TypeScriptConfigurationParser_1.parseTypeScriptConfiguration(typescript, configuration.configFile, configuration.context, configuration.configOverwrite, parseConfigFileHost);
                performance.markEnd('Parse Configuration');
                if (parsedConfiguration.errors) {
                    parseConfigurationDiagnostics.push(...parsedConfiguration.errors);
                }
                // report configuration diagnostics and exit
                if (parseConfigurationDiagnostics.length) {
                    parsedConfiguration = undefined;
                    let issues = TypeScriptIssueFactory_1.createIssuesFromTsDiagnostics(typescript, parseConfigurationDiagnostics);
                    issues.forEach((issue) => {
                        if (!issue.file) {
                            issue.file = configuration.configFile;
                        }
                    });
                    extensions.forEach((extension) => {
                        if (extension.extendIssues) {
                            issues = extension.extendIssues(issues);
                        }
                    });
                    return issues;
                }
                if (configurationChanged) {
                    configurationChanged = false;
                    // try to remove outdated .tsbuildinfo file for incremental mode
                    if (typeof typescript.getTsBuildInfoEmitOutputFilePath === 'function' &&
                        configuration.mode !== 'readonly' &&
                        parsedConfiguration.options.incremental) {
                        const tsBuildInfoPath = typescript.getTsBuildInfoEmitOutputFilePath(parsedConfiguration.options);
                        if (tsBuildInfoPath) {
                            try {
                                system.deleteFile(tsBuildInfoPath);
                            }
                            catch (error) {
                                // silent
                            }
                        }
                    }
                }
            }
            if (configuration.build) {
                // solution builder case
                // ensure watch solution builder host exists
                if (!watchSolutionBuilderHost) {
                    performance.markStart('Create Solution Builder Host');
                    watchSolutionBuilderHost = ControlledWatchSolutionBuilderHost_1.createControlledWatchSolutionBuilderHost(typescript, parsedConfiguration, system, typescript.createSemanticDiagnosticsBuilderProgram, undefined, undefined, undefined, undefined, (builderProgram) => {
                        const projectName = getProjectNameOfBuilderProgram(builderProgram);
                        const diagnostics = getDiagnosticsOfBuilderProgram(builderProgram);
                        // update diagnostics
                        diagnosticsPerProject.set(projectName, diagnostics);
                        // emit .tsbuildinfo file if needed
                        emitTsBuildInfoFileForBuilderProgram(builderProgram);
                    }, extensions);
                    performance.markEnd('Create Solution Builder Host');
                    solutionBuilder = undefined;
                }
                // ensure solution builder exists
                if (!solutionBuilder) {
                    performance.markStart('Create Solution Builder');
                    solutionBuilder = typescript.createSolutionBuilderWithWatch(watchSolutionBuilderHost, [configuration.configFile], {});
                    performance.markEnd('Create Solution Builder');
                    performance.markStart('Build Solutions');
                    solutionBuilder.build();
                    performance.markEnd('Build Solutions');
                }
            }
            else {
                // watch compiler case
                // ensure watch compiler host exists
                if (!watchCompilerHost) {
                    performance.markStart('Create Watch Compiler Host');
                    watchCompilerHost = ControlledWatchCompilerHost_1.createControlledWatchCompilerHost(typescript, parsedConfiguration, system, typescript.createSemanticDiagnosticsBuilderProgram, undefined, undefined, (builderProgram) => {
                        const projectName = getProjectNameOfBuilderProgram(builderProgram);
                        const diagnostics = getDiagnosticsOfBuilderProgram(builderProgram);
                        // update diagnostics
                        diagnosticsPerProject.set(projectName, diagnostics);
                        // emit .tsbuildinfo file if needed
                        emitTsBuildInfoFileForBuilderProgram(builderProgram);
                    }, extensions);
                    performance.markEnd('Create Watch Compiler Host');
                    watchProgram = undefined;
                }
                // ensure watch program exists
                if (!watchProgram) {
                    performance.markStart('Create Watch Program');
                    watchProgram = typescript.createWatchProgram(watchCompilerHost);
                    performance.markEnd('Create Watch Program');
                }
            }
            performance.markStart('Poll And Invoke Created Or Deleted');
            system.pollAndInvokeCreatedOrDeleted();
            performance.markEnd('Poll And Invoke Created Or Deleted');
            changedFiles.forEach((changedFile) => {
                if (system) {
                    system.invokeFileChanged(changedFile);
                }
            });
            deletedFiles.forEach((removedFile) => {
                if (system) {
                    system.invokeFileDeleted(removedFile);
                }
            });
            // wait for all queued events to be processed
            performance.markStart('Queued Tasks');
            yield system.waitForQueued();
            performance.markEnd('Queued Tasks');
            // aggregate all diagnostics and map them to issues
            const diagnostics = [];
            diagnosticsPerProject.forEach((projectDiagnostics) => {
                diagnostics.push(...projectDiagnostics);
            });
            let issues = TypeScriptIssueFactory_1.createIssuesFromTsDiagnostics(typescript, diagnostics);
            extensions.forEach((extension) => {
                if (extension.extendIssues) {
                    issues = extension.extendIssues(issues);
                }
            });
            if (configuration.profile) {
                performance.print();
                performance.disable();
            }
            return issues;
        }),
    };
}
exports.createTypeScriptReporter = createTypeScriptReporter;
