"use strict";
Object.defineProperty(exports, "__esModule", { value: true });
function parseTypeScriptConfiguration(typescript, configFileName, configFileContext, configOverwriteJSON, parseConfigFileHost) {
    const parsedConfigFileJSON = typescript.readConfigFile(configFileName, parseConfigFileHost.readFile);
    const overwrittenConfigFileJSON = Object.assign(Object.assign(Object.assign({}, (parsedConfigFileJSON.config || {})), configOverwriteJSON), { compilerOptions: Object.assign(Object.assign({}, ((parsedConfigFileJSON.config || {}).compilerOptions || {})), (configOverwriteJSON.compilerOptions || {})) });
    const parsedConfigFile = typescript.parseJsonConfigFileContent(overwrittenConfigFileJSON, parseConfigFileHost, configFileContext);
    return Object.assign(Object.assign({}, parsedConfigFile), { options: Object.assign(Object.assign({}, parsedConfigFile.options), { configFilePath: configFileName }), errors: parsedConfigFileJSON.error ? [parsedConfigFileJSON.error] : parsedConfigFile.errors });
}
exports.parseTypeScriptConfiguration = parseTypeScriptConfiguration;
