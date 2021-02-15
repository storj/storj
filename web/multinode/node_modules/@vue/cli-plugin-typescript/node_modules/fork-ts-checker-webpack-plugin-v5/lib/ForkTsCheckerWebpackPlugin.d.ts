import webpack from 'webpack';
import { ForkTsCheckerWebpackPluginOptions } from './ForkTsCheckerWebpackPluginOptions';
declare class ForkTsCheckerWebpackPlugin implements webpack.Plugin {
    static readonly version: string;
    private readonly options;
    constructor(options?: ForkTsCheckerWebpackPluginOptions);
    static getCompilerHooks(compiler: webpack.Compiler): {
        start: import("tapable").AsyncSeriesWaterfallHook<import("./reporter/FilesChange").FilesChange, webpack.compilation.Compilation, any>;
        waiting: import("tapable").SyncHook<webpack.compilation.Compilation, any, any>;
        canceled: import("tapable").SyncHook<webpack.compilation.Compilation, any, any>;
        error: import("tapable").SyncHook<Error, webpack.compilation.Compilation, any>;
        issues: import("tapable").SyncWaterfallHook<import("./issue/Issue").Issue[], webpack.compilation.Compilation | undefined, void>;
    };
    apply(compiler: webpack.Compiler): void;
}
export { ForkTsCheckerWebpackPlugin };
