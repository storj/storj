# 1.16.0

**Features:**

* `naming-convention`: `accessor` is now configurable

# 1.15.1

**Bugfixes:**

* `naming-convention`: `filter` option is no longer affect by the order of checked names and caching

# 1.15.0

**Features:**

* `no-unnecessary-else` and `no-else-after-return` are now automatically fixable

# 1.14.1

**Bugfixes:**

* fixed version of `tsutils` dependency

# 1.14.0

**Features:**

* `early-exit`: added option `"ignore-constructor"`

# 1.13.3

**Bugfixes:**

* `no-unused`: don't mark import of `React` as unused if required by JsxFragment

# 1.13.2

**Bugfixes:**

* `no-unused`: fixed crash on type parameter of declaration inside global augmentation

# 1.13.1

**Bugfixes:**

* `naming-convention`: added missing configuration handling for `"type": "parameterProperty"`

# 1.13.0

**Features:**

* `no-else-after-return`: added option `"allow-else-if"`

# 1.12.3

* `no-else-after-return`: use correct control flow analysis

# 1.12.2

* `no-unnecessary-type-annotation`: fix infinite loop on union with generic signature (#57)

# 1.12.1

**Bugfixes:**

* `no-unnecessary-type-annotations` now recognizes IIFEs that look like `(function(param: string) {}('parameter'));`

# 1.12.0

**Features:**

This package now provides all rules for the [Wotan linter runtime](https://github.com/fimbullinter/wotan/blob/master/packages/wotan/README.md) as well.
Example `.wotanrc.yaml`:

```yaml
extends:
  - tslint-consistent-codestyle # makes rules from the package available with the 'tcc/' prefix
rules: # now configure the rules you want to use, remember to use the 'tcc/' prefix
  tcc/no-collapsible-if: error
  tcc/no-unused:
    options: 'ignore-parameters'
```

# 1.11.1

**Bugfixes:**

* `ext-curly`: handle LabeledStatement as if it was not present

# 1.11.0

**Non-breaking rule changes:** (you might want to amend your configuration to make the rules stricter again)

* `naming-convention`: Format options `"camelCase"` and `"PascalCase"` now allow adjacent uppercase characters to make it usable for real-world code. Use the new options `"strictCamelCase"` and `"StrictPascalCase"` to restore the old strict behavior.
* `no-unnecessary-type-annotation`: disabled checking return type annotations by default as these cause the most false positves. You can enable the check with the new option `"check-return-type"`.

**Bugfixes:**

* `naming-convention`: fixed a bug where the suffix was not correctly removed from the name when using an array of suffixes.

**Features:**

* `naming-convention`: added new type `"functionVariable"`.
  * It matches all variables initialized with an arrow function or function expression.
  * It inherits configuration from `"variable"`

# 1.10.2

**Bugfixes:**

* `no-accessor-recursion` fixed crash on abstract accessor

# 1.10.1

**Bugfixes:**

* `no-unused` added special handling for `React` implicitly used by JSX

# 1.10.0

**Bugfixes:**

* `no-unnecessary-type-annotation`
  * avoid infinite recursion for circular type parameters
  * fix signature arity calculation

**Features:**

* new rule [`no-accessor-recursion`](https://github.com/ajafff/tslint-consistent-codestyle/blob/master/docs/no-accessor-recursion.md)

# 1.9.3

**Bugfixes:**

* `no-unnecessary-type-annotation`
  * choose correct signature by arity
  * correctly handle methods with numeric name
  * correctly handle methods with computed names

# 1.9.2

**Bugfixes:**

* `no-unnecessary-type-annotation`
  * fixed a false positive with typeguard functions
  * this rule is no longer considered experimental

# 1.9.1

**Bugfixes:**

* `no-unnecessary-type-annotation`
  * check return types
  * check types in IIFE (immediately invoked function expression)
  * check object literal methods with contextual type
  * exempt `this` parameters
  * correctly handle optional parameters
  * correctly handle rest parameters

# 1.9.0

**Features:**

* new *experimental* rule [`no-unnecessary-type-annotation`](https://github.com/ajafff/tslint-consistent-codestyle/blob/master/docs/no-unnecessary-type-annotation.md)
* `naming-convention` adds `unused` modifier -> this enables you to allow leading underscore only for unused variables and parameters
* `no-var-before-return` adds option `"allow-destructuring"`
* `early-exit` adds special handling for `{"max-length": 0}`

# 1.8.0

**Bugfixes:**

* `no-as-type-assertion`
  * Insert fix at the correct position
  * Fixed ordering issues in fixer
  * The rule disables itself in `.jsx` and `.tsx` files

**Features:**

* `no-unused` adds option `"unused-catch-binding"` to disallow unused catch bindings. Only use this rule if you use TypeScript@2.5.1 or newer

# 1.7.0

**Features:**

* new rule [`const-parameters`](https://github.com/ajafff/tslint-consistent-codestyle/blob/master/docs/const-parameters.md)

# 1.6.0

**Bugfixes:**

* `prefer-const-enum` bail on string to number conversion
* `no-unused` fixed false positive with index signature

**Features:**

* `parameter-properties` adds `"trailing"` option

# 1.5.1

**Bugfixes:**

* `no-var-before-return` now detects if variable is used by a closure.
* `prefer-const-enum` is now stable:
  * Correct handling of scopes
  * Handle enums merging with namespace
  * Exclude enums in global scope
  * Handle string valued enums
  * Bugfix for enum used as type
  * Stricter checks for initializer

# 1.5.0

**Features:**

* :sparkles: New rule [`no-unused`](https://github.com/ajafff/tslint-consistent-codestyle/blob/master/docs/no-unused.md) to find dead code and unused declarations.
* New rule [`early-exit`](https://github.com/ajafff/tslint-consistent-codestyle/blob/master/docs/early-exit.md) recommends to use an early exit instead of a long `if` block. Big thanks to @andy-hanson for this great contribution.

# 1.4.0

**Features:**

* New rule `ext-curly` to enforce consistent use of curly braces.

**Bugfixes:**

* `no-var-before-return` now has an exception array destructuring because there could be an iterator being destructured.

# 1.3.0

**Features:**

* This package now contains an empty config that can easily be extended using `"extends": ["tslint-consistent-codestyle"]` in your `tslint.json`
* Add documentation about the module resolution of `rulesDirectory`

**Bugfixes:**

* Remove `no-curly` rule from package, which is still under development

# 1.2.0

**Features:**

* `naming-convention`: Allow an array of formats

**Bugfixes:**

* `naming-convention`:
  * `global` modifier now works correctly on functions, classes, enums, etc. Before they were all considered `local`
  * type `function` now correctly inherits from type `variable` instead of getting overridden depending on their ordering
  * Adding a `filter` to a configuration no longer overrides every other config in the inheritance chain

# 1.1.0

**Features:**

* `naming-convention`: Add `filter` option to config

# 1.0.0

**Breaking Changes:**

* Update to tslint@5
* Removed `prefer-static-method`, use tslint's `prefer-function-over-method` instead
* PascalCase and camelCase can no longer contain two adjacent uppercase characters
* UPPER_CASE and snake_case can no longer contain two adjacent underscores

**Bugfixes:**

* Exempt `this` parameter from name checks
