import webpack from 'webpack';
import { IssuePredicate } from './IssuePredicate';
import { IssueOptions } from './IssueOptions';
interface IssueConfiguration {
    predicate: IssuePredicate;
    scope: 'all' | 'webpack';
}
declare function createIssueConfiguration(compiler: webpack.Compiler, options: IssueOptions | undefined): IssueConfiguration;
export { IssueConfiguration, createIssueConfiguration };
