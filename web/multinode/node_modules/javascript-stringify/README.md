# JavaScript Stringify

[![NPM version][npm-image]][npm-url]
[![NPM downloads][downloads-image]][downloads-url]
[![Build status][travis-image]][travis-url]
[![Test coverage][coveralls-image]][coveralls-url]

> Stringify is to `eval` as `JSON.stringify` is to `JSON.parse`.

## Installation

```
npm install javascript-stringify --save
```

---

<a href="https://www.buymeacoffee.com/blakeembrey" target="_blank"><img src="https://www.buymeacoffee.com/assets/img/custom_images/orange_img.png" alt="Buy Me A Coffee" ></a>

## Usage

```javascript
import { stringify } from "javascript-stringify";
```

The API is similar `JSON.stringify`:

- `value` The value to convert to a string
- `replacer` A function that alters the behavior of the stringification process
- `space` A string or number that's used to insert white space into the output for readability purposes
- `options`
  - **maxDepth** _(number, default: 100)_ The maximum depth of values to stringify
  - **maxValues** _(number, default: 100000)_ The maximum number of values to stringify
  - **references** _(boolean, default: false)_ Restore circular/repeated references in the object (uses IIFE)
  - **skipUndefinedProperties** _(boolean, default: false)_ Omits `undefined` properties instead of restoring as `undefined`

### Examples

```javascript
stringify({}); // "{}"
stringify(true); // "true"
stringify("foo"); // "'foo'"

stringify({ x: 5, y: 6 }); // "{x:5,y:6}"
stringify([1, 2, 3, "string"]); // "[1,2,3,'string']"

stringify({ a: { b: { c: 1 } } }, null, null, { maxDepth: 2 }); // "{a:{b:{}}}"

/**
 * Invalid key names are automatically stringified.
 */

stringify({ "some-key": 10 }); // "{'some-key':10}"

/**
 * Some object types and values can remain identical.
 */

stringify([/.+/gi, new Number(10), new Date()]); // "[/.+/gi,new Number(10),new Date(1406623295732)]"

/**
 * Unknown or circular references are removed.
 */

var obj = { x: 10 };
obj.circular = obj;

stringify(obj); // "{x:10}"
stringify(obj, null, null, { references: true }); // "(function(){var x={x:10};x.circular=x;return x;}())"

/**
 * Specify indentation - just like `JSON.stringify`.
 */

stringify({ a: 2 }, null, " "); // "{\n a: 2\n}"
stringify({ uno: 1, dos: 2 }, null, "\t"); // "{\n\tuno: 1,\n\tdos: 2\n}"

/**
 * Add custom replacer behaviour - like double quoted strings.
 */

stringify(["test", "string"], function(value, indent, stringify) {
  if (typeof value === "string") {
    return '"' + value.replace(/"/g, '\\"') + '"';
  }

  return stringify(value);
});
//=> '["test","string"]'
```

## License

MIT

[npm-image]: https://img.shields.io/npm/v/javascript-stringify.svg?style=flat
[npm-url]: https://npmjs.org/package/javascript-stringify
[downloads-image]: https://img.shields.io/npm/dm/javascript-stringify.svg?style=flat
[downloads-url]: https://npmjs.org/package/javascript-stringify
[travis-image]: https://img.shields.io/travis/blakeembrey/javascript-stringify.svg?style=flat
[travis-url]: https://travis-ci.org/blakeembrey/javascript-stringify
[coveralls-image]: https://img.shields.io/coveralls/blakeembrey/javascript-stringify.svg?style=flat
[coveralls-url]: https://coveralls.io/r/blakeembrey/javascript-stringify?branch=master
