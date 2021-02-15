/** @typedef {import('webpack').Compiler} Compiler */
/** @typedef {import('./getOptions').Options} Options */
/** @typedef {import('./linter').Lint} Lint */
/** @typedef {import('./linter').LinterCallback} LinterCallback */
/** @typedef {Partial<{timestamp:number} | number>} FileSystemInfoEntry */
export default class LintDirtyModulesPlugin {
  /**
   * @param {Lint} lint
   * @param {Compiler} compiler
   * @param {Options} options
   */
  constructor(lint: Lint, compiler: Compiler, options: Options);
  lint: import('./linter').Lint;
  compiler: import('webpack').Compiler;
  options: import('./getOptions').Options;
  startTime: number;
  prevTimestamps: Map<any, any>;
  isFirstRun: boolean;
  /**
   * @param {Compiler} compilation
   * @param {LinterCallback} callback
   * @returns {void}
   */
  apply(compilation: Compiler, callback: LinterCallback): void;
  /**
   * @param {Map<string, number|FileSystemInfoEntry>} fileTimestamps
   * @param {string | ReadonlyArray<string>} glob
   * @returns {Array<string>}
   */
  getChangedFiles(
    fileTimestamps: Map<
      string,
      | number
      | Partial<{
          timestamp: number;
        }>
    >,
    glob: string | ReadonlyArray<string>
  ): Array<string>;
}
export type Compiler = import('webpack').Compiler;
export type Options = {
  context?: string | undefined;
  emitError?: boolean | undefined;
  emitWarning?: boolean | undefined;
  failOnError?: boolean | undefined;
  failOnWarning?: boolean | undefined;
  files: string | string[];
  formatter: TimerHandler;
  lintDirtyModulesOnly?: boolean | undefined;
  quiet?: boolean | undefined;
  stylelintPath: string;
};
export type Lint = (
  options: import('./getOptions').Options
) => Promise<import('stylelint').LinterResult>;
export type LinterCallback = (
  error?: import('./StylelintError').default | null | undefined
) => void;
export type FileSystemInfoEntry =
  | number
  | Partial<{
      timestamp: number;
    }>;
