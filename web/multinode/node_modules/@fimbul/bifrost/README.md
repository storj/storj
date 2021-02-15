# Bifröst

Compatiblity layer for TSLint rules and formatters.

[![npm version](https://img.shields.io/npm/v/@fimbul/bifrost.svg)](https://www.npmjs.com/package/@fimbul/bifrost)
[![npm downloads](https://img.shields.io/npm/dm/@fimbul/bifrost.svg)](https://www.npmjs.com/package/@fimbul/bifrost)
[![Renovate enabled](https://img.shields.io/badge/renovate-enabled-brightgreen.svg)](https://renovateapp.com/)
[![CircleCI](https://circleci.com/gh/fimbullinter/wotan/tree/master.svg?style=shield)](https://circleci.com/gh/fimbullinter/wotan/tree/master)
[![Build status](https://ci.appveyor.com/api/projects/status/a28dpupxvjljibq3/branch/master?svg=true)](https://ci.appveyor.com/project/ajafff/wotan/branch/master)
[![codecov](https://codecov.io/gh/fimbullinter/wotan/branch/master/graph/badge.svg)](https://codecov.io/gh/fimbullinter/wotan)
[![Join the chat at https://gitter.im/fimbullinter/wotan](https://badges.gitter.im/fimbullinter/wotan.svg)](https://gitter.im/fimbullinter/wotan)

Make sure to also read the [full documentation of all available modules](https://github.com/fimbullinter/wotan#readme).

## Purpose

Allows TSLint rule authors to provide the same rules for Wotan without any refactoring.
Although `@fimbul/heimdall` already allows users to use your rules and formatters in Wotan, they still need to remember to use `-m @fimbul/heimdall` when running Wotan.
You can help these users by providing your rules in a format that Wotan understands without any plugin.

It also provides the exact opposite functionality: using rules originally written for Fimbullinter (wotan) as TSLint rule.

## Installation

```sh
npm install --save @fimbul/bifrost
# or
yarn add @fimbul/bifrost
```

## Using TSLint Rules and Formatters in Wotan

### Rules

Given a TSLint rule `my-foo` in a file `myFooRule.ts`, you simply create a file `my-foo.ts` with the following content:

```ts
import {wrapTslintRule} from '@fimbul/bifrost';
import {Rule} from './myFooRule.ts';

const Wrapped = wrapTslintRule(Rule, 'my-foo');
export {Wrapped as Rule};
```

If you want to use a different directory for your TSLint rules and their Wotan wrapper, you just need to adjust the paths in the above example.

### Formatters

Given a TSLint formatter `my-foo` in a file `myFooFormatter.ts`, you simply create a file `my-foo.ts` with the following content:

```ts
import {wrapTslintFormatter} from '@fimbul/bifrost';
import {Formatter} from './myFooFormatter.ts';

const Wrapped = wrapTslintFormatter(Formatter);
export {Wrapped as Formatter};
```

Note that findings with severity `suggestion` are reported as `warning` through TSLint formatters.

## Using Fimbullinter Rules in TSLint

Given a Fimbullinter rule `my-foo` in a file `my-foo.ts`, you simply create a file `myFooRule.ts` with the following content:

```ts
import {wrapRuleForTslint} from '@fimbul/bifrost';
import {Rule} from './my-foo.ts';

const Wrapped = wrapRuleForTslint(Rule);
export {Wrapped as Rule};
```

## License

Apache-2.0 © [Klaus Meinhardt](https://github.com/ajafff)
