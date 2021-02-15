/** @typedef {import('stylelint').LintResult} LintResult */
/** @typedef {import('./getOptions').Options} Options */
export default class StylelintError extends Error {
  /**
   * @param {Options} options
   * @param {Array<LintResult>} messages
   * @returns {StylelintError}
   */
  static format(
    { formatter }: Options,
    messages: Array<LintResult>
  ): StylelintError;
  /**
   * @param {Partial<string>} messages
   */
  constructor(messages: Partial<string>);
}
export type LintResult = import('stylelint').LintResult;
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
