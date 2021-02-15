import webpack from 'webpack';
import { ForkTsCheckerWebpackPluginConfiguration } from '../ForkTsCheckerWebpackPluginConfiguration';
declare function tapAfterCompileToAddDependencies(compiler: webpack.Compiler, configuration: ForkTsCheckerWebpackPluginConfiguration): void;
export { tapAfterCompileToAddDependencies };
