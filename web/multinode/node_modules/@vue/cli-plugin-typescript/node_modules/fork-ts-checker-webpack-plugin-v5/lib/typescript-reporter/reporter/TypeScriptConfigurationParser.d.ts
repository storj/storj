import * as ts from 'typescript';
import { TypeScriptConfigurationOverwrite } from '../TypeScriptConfigurationOverwrite';
declare function parseTypeScriptConfiguration(typescript: typeof ts, configFileName: string, configFileContext: string, configOverwriteJSON: TypeScriptConfigurationOverwrite, parseConfigFileHost: ts.ParseConfigFileHost): ts.ParsedCommandLine;
export { parseTypeScriptConfiguration };
