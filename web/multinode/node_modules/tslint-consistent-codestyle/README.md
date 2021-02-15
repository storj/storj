[![npm version](http://img.shields.io/npm/v/tslint-consistent-codestyle.svg)](https://npmjs.org/package/tslint-consistent-codestyle)
[![Downloads](http://img.shields.io/npm/dm/tslint-consistent-codestyle.svg)](https://npmjs.org/package/tslint-consistent-codestyle)
[![CircleCI](https://circleci.com/gh/ajafff/tslint-consistent-codestyle.svg?style=shield)](https://circleci.com/gh/ajafff/tslint-consistent-codestyle)
[![Coverage Status](https://coveralls.io/repos/github/ajafff/tslint-consistent-codestyle/badge.svg)](https://coveralls.io/github/ajafff/tslint-consistent-codestyle)
[![Join the chat at https://gitter.im/ajafff/tslint-consistent-codestyle](https://badges.gitter.im/ajafff/tslint-consistent-codestyle.svg)](https://gitter.im/ajafff/tslint-consistent-codestyle?utm_source=badge&utm_medium=badge&utm_campaign=pr-badge&utm_content=badge)
[![Greenkeeper badge](https://badges.greenkeeper.io/ajafff/tslint-consistent-codestyle.svg)](https://greenkeeper.io/)

# Purpose

The rules in this package can be used to enforce consistent code style.

# Usage

Install from npm to your devDependencies  (https://www.npmjs.com/package/tslint-consistent-codestyle)

```sh
npm install --save-dev tslint-consistent-codestyle
```

## With TSLint

Configure tslint to use `tslint-consistent-codestyle`:

This package provides an empty configuration preset that just contains the `rulesDirectory`. That means you can easily use the rules in this package, but don't get any predefined configuration. To use it, just add it to the `extends` array in your `tslint.json`:

```javascript
{
   "extends": ["tslint-consistent-codestyle"]
   "rules": {
     ...
   }
}
```

As of `tslint@5.2.0` you can also use `tslint-consistent-codestyle` as `rulesDirectory`:

```javascript
{
   "rulesDirectory": ["tslint-consistent-codestyle"]
   "rules": {
     ...
   }
}
```

Now configure some of the new rules.

## With Wotan

This package provides all rules for both TSLint and [Wotan](https://github.com/fimbullinter/wotan/blob/master/packages/wotan/README.md).

To use rules from this package, add the following to your `.wotanrc.yaml` file:

```yaml
extends:
  - tslint-consistent-codestyle # makes rules from the package available with the 'tcc/' prefix
rules: # now configure the rules you want to use, remember to use the 'tcc/' prefix
  tcc/no-collapsible-if: error
  tcc/no-unused:
    options: 'ignore-parameters'
```

# Rules

Rule | Description
---- | ----
[const-parameters](https://github.com/ajafff/tslint-consistent-codestyle/blob/master/docs/const-parameters.md) | Declare parameters as `const` with JsDoc `/** @const */`
[early-exit](https://github.com/ajafff/tslint-consistent-codestyle/blob/master/docs/early-exit.md) | Recommends to use an early exit instead of a long `if` block.
[ext-curly](https://github.com/ajafff/tslint-consistent-codestyle/blob/master/docs/ext-curly.md) | Enforces where to consistently use curly braces where not strictly necessary.
[naming-convention](https://github.com/ajafff/tslint-consistent-codestyle/blob/master/docs/naming-convention.md) | Fine grained configuration to enforce consistent naming for almost everything. E.g. variables, functions, classes, methods, parameters, enums, etc.
[no-as-type-assertion](https://github.com/ajafff/tslint-consistent-codestyle/blob/master/docs/no-as-type-assertion.md) | Prefer `<Type>foo` over `foo as Type`.
[no-accessor-recursion](https://github.com/ajafff/tslint-consistent-codestyle/blob/master/docs/no-accessor-recursion.md) | Don't use `get foo() { return this.foo; }`. This is most likely a typo.
[no-collapsible-if](https://github.com/ajafff/tslint-consistent-codestyle/blob/master/docs/no-collapsible-if.md) | Identifies nested if statements that can be combined into one.
[no-else-after-return](https://github.com/ajafff/tslint-consistent-codestyle/blob/master/docs/no-else-after-return.md) | Like [no-else-return from eslint](http://eslint.org/docs/rules/no-else-return).
[no-return-undefined](https://github.com/ajafff/tslint-consistent-codestyle/blob/master/docs/no-return-undefined.md) | Just `return;` instead of `return undefined;`.
[no-static-this](https://github.com/ajafff/tslint-consistent-codestyle/blob/master/docs/no-static-this.md) | Ban the use of `this` in static methods.
[no-unnecessary-else](https://github.com/ajafff/tslint-consistent-codestyle/blob/master/docs/no-unnecessary-else.md) | Like `no-else-after-return` but better.
[no-unnecessary-type-annotation](https://github.com/ajafff/tslint-consistent-codestyle/blob/master/docs/no-unnecessary-type-annotation.md) | Finds type annotations that can safely be removed.
[no-unused](https://github.com/ajafff/tslint-consistent-codestyle/blob/master/docs/no-unused.md) | Find dead code and unused declarations.
[no-var-before-return](https://github.com/ajafff/tslint-consistent-codestyle/blob/master/docs/no-var-before-return.md) | Checks if the returned variable is declared right before the `return` statement.
[object-shorthand-properties-first](https://github.com/ajafff/tslint-consistent-codestyle/blob/master/docs/object-shorthand-properties-first.md) | Shorthand properties should precede regular properties.
[parameter-properties](https://github.com/ajafff/tslint-consistent-codestyle/blob/master/docs/parameter-properties.md) | Configure how and where to declare parameter properties.
[prefer-const-enum](https://github.com/ajafff/tslint-consistent-codestyle/blob/master/docs/prefer-const-enum.md) | Prefer `const enum` where possible.
[prefer-while](https://github.com/ajafff/tslint-consistent-codestyle/blob/master/docs/prefer-while.md) | Prefer a `while` loop instead of a `for` loop without initializer and incrementer.
