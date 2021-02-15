import webpack from 'webpack';
import { ForkTsCheckerWebpackPluginState } from '../ForkTsCheckerWebpackPluginState';
declare function getDeletedFiles(compiler: webpack.Compiler, state: ForkTsCheckerWebpackPluginState): string[];
export { getDeletedFiles };
