import webpack = require('webpack');
import { TemplateCompiler, CompilerOptions, SFCTemplateCompileOptions, SFCScriptCompileOptions } from '@vue/compiler-sfc';
import VueLoaderPlugin from './plugin';
export { VueLoaderPlugin };
export interface VueLoaderOptions {
    babelParserPlugins?: SFCScriptCompileOptions['babelParserPlugins'];
    transformAssetUrls?: SFCTemplateCompileOptions['transformAssetUrls'];
    compiler?: TemplateCompiler | string;
    compilerOptions?: CompilerOptions;
    hotReload?: boolean;
    exposeFilename?: boolean;
    appendExtension?: boolean;
    isServerBuild?: boolean;
}
export default function loader(this: webpack.loader.LoaderContext, source: string): string | void;
