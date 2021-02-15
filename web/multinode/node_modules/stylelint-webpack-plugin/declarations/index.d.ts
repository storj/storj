export default StylelintWebpackPlugin;
export type Compiler = import('webpack').Compiler;
/** @typedef {import('webpack').Compiler} Compiler */
declare class StylelintWebpackPlugin {
  constructor(options?: {});
  options: import('./getOptions').Options;
  /**
   * @param {Compiler} compiler
   * @returns {void}
   */
  apply(compiler: Compiler): void;
  /**
   *
   * @param {Compiler} compiler
   * @returns {string}
   */
  getContext(compiler: Compiler): string;
}
