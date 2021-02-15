/** @typedef {import('stylelint').LinterResult} LinterResult */
/** @typedef {import('stylelint').LintResult} LintResult */
/** @typedef {import('webpack').Compiler} Compiler */
/** @typedef {import('./getOptions').Options} Options */
/**
 * @callback Lint
 * @param {Options} options
 * @returns {Promise<LinterResult>}
 */
/**
 * @callback LinterCallback
 * @param {StylelintError | null=} error
 * @returns {void}
 */
/**
 * @param {Lint} lint
 * @param {Options} options
 * @param {Compiler} compiler
 * @param {LinterCallback} callback
 * @returns {void}
 */
export default function linter(
  lint: Lint,
  options: Options,
  compiler: Compiler,
  callback: LinterCallback
): void;
export type LinterResult = import('stylelint').LinterResult;
export type LintResult = import('stylelint').LintResult;
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
export type Lint = (options: Options) => Promise<LinterResult>;
export type LinterCallback = (
  error?: (StylelintError | null) | undefined
) => void;
import StylelintError from './StylelintError';
