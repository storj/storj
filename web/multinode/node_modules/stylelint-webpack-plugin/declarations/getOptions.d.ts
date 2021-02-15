/** @typedef {import("stylelint")} stylelint */
/**
 * @typedef {Object} Options
 * @property {string=} context
 * @property {boolean=} emitError
 * @property {boolean=} emitWarning
 * @property {boolean=} failOnError
 * @property {boolean=} failOnWarning
 * @property {Array<string> | string} files
 * @property {Function | string} formatter
 * @property {boolean=} lintDirtyModulesOnly
 * @property {boolean=} quiet
 * @property {string} stylelintPath
 */
/**
 * @param {Partial<Options>} pluginOptions
 * @returns {Options}
 */
export default function getOptions(pluginOptions: Partial<Options>): Options;
export type stylelint = typeof import('stylelint');
export type Options = {
  context?: string | undefined;
  emitError?: boolean | undefined;
  emitWarning?: boolean | undefined;
  failOnError?: boolean | undefined;
  failOnWarning?: boolean | undefined;
  files: Array<string> | string;
  formatter: Function | string;
  lintDirtyModulesOnly?: boolean | undefined;
  quiet?: boolean | undefined;
  stylelintPath: string;
};
