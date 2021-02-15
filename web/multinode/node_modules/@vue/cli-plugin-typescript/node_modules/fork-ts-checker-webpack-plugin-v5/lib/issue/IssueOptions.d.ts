import { IssueMatch } from './IssueMatch';
import { IssuePredicate } from './IssuePredicate';
declare type IssuePredicateOption = IssuePredicate | IssueMatch | (IssuePredicate | IssueMatch)[];
interface IssueOptions {
    include?: IssuePredicateOption;
    exclude?: IssuePredicateOption;
    scope?: 'all' | 'webpack';
}
export { IssueOptions, IssuePredicateOption };
