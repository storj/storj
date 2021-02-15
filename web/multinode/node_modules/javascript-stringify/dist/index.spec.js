"use strict";
Object.defineProperty(exports, "__esModule", { value: true });
const fc = require("fast-check");
const semver_1 = require("semver");
const index_1 = require("./index");
// Evaluate a string into JavaScript
const evalValue = (str) => {
    // tslint:disable-next-line no-eval
    return eval(`(${str})`);
};
/**
 * Create a quick test function wrapper.
 */
function test(value, result, indent, options) {
    return () => {
        expect(index_1.stringify(value, null, indent, options)).toEqual(result);
    };
}
/**
 * Create a wrapper for round-trip eval tests.
 */
function testRoundTrip(expression, indent, options) {
    return () => test(evalValue(expression), expression, indent, options)();
}
/**
 * Check if syntax is supported.
 */
function isSupported(expr) {
    try {
        // tslint:disable-next-line no-eval
        eval(expr);
        return true;
    }
    catch (err) {
        if (err.name === "SyntaxError")
            return false;
        throw err;
    }
}
/**
 * Generate a list of test cases to run.
 */
function cases(cases) {
    return () => {
        for (const value of cases) {
            if (value)
                it(value, testRoundTrip(value));
        }
    };
}
/**
 * Conditionally execute test cases.
 */
function describeIf(description, condition, fn) {
    return condition ? describe(description, fn) : describe.skip(description, fn);
}
describe("javascript-stringify", () => {
    describe("types", () => {
        describe("booleans", () => {
            it("should be stringified", test(true, "true"));
        });
        describe("strings", () => {
            it("should wrap in single quotes", test("string", "'string'"));
            it("should escape quote characters", test("'test'", "'\\'test\\''"));
            it("should escape control characters", test("multi\nline", "'multi\\nline'"));
            it("should escape back slashes", test("back\\slash", "'back\\\\slash'"));
            it("should escape certain unicode sequences", test("\u0602", "'\\u0602'"));
        });
        describe("numbers", () => {
            it("should stringify integers", test(10, "10"));
            it("should stringify floats", test(10.5, "10.5"));
            it('should stringify "NaN"', test(10.5, "10.5"));
            it('should stringify "Infinity"', test(Infinity, "Infinity"));
            it('should stringify "-Infinity"', test(-Infinity, "-Infinity"));
            it('should stringify "-0"', test(-0, "-0"));
        });
        describe("arrays", () => {
            it("should stringify as array shorthand", test([1, 2, 3], "[1,2,3]"));
            it("should indent elements", test([{ x: 10 }], "[\n\t{\n\t\tx: 10\n\t}\n]", "\t"));
        });
        describe("objects", () => {
            it("should stringify as object shorthand", test({ key: "value", "-": 10 }, "{key:'value','-':10}"));
            it("should stringify undefined keys", test({ a: true, b: undefined }, "{a:true,b:undefined}"));
            it("should stringify omit undefined keys", test({ a: true, b: undefined }, "{a:true}", null, {
                skipUndefinedProperties: true
            }));
            it("should quote reserved word keys", test({ if: true, else: false }, "{'if':true,'else':false}"));
            it("should not quote Object.prototype keys", test({ constructor: 1, toString: 2 }, "{constructor:1,toString:2}"));
        });
        describe("functions", () => {
            it("should reindent function bodies", test(evalValue(`function() {
              if (true) {
                return "hello";
              }
            }`), 'function () {\n  if (true) {\n    return "hello";\n  }\n}', 2));
            it("should reindent function bodies in objects", test(evalValue(`
          {
            fn: function() {
              if (true) {
                return "hello";
              }
            }
          }
          `), '{\n  fn: function () {\n    if (true) {\n      return "hello";\n    }\n  }\n}', 2));
            it("should reindent function bodies in arrays", test(evalValue(`[
            function() {
              if (true) {
                return "hello";
              }
            }
          ]`), '[\n  function () {\n    if (true) {\n      return "hello";\n    }\n  }\n]', 2));
            it("should not need to reindent one-liners", testRoundTrip("{\n  fn: function () { return; }\n}", 2));
            it("should gracefully handle unexpected Function.toString formats", () => {
                const origToString = Function.prototype.toString;
                Function.prototype.toString = () => "{nope}";
                try {
                    expect(index_1.stringify(function () {
                        /* Empty */
                    })).toEqual("void '{nope}'");
                }
                finally {
                    Function.prototype.toString = origToString;
                }
            });
            describe("omit the names of their keys", cases(["{name:function () {}}", "{'tricky name':function () {}}"]));
        });
        describe("native instances", () => {
            describe("Date", () => {
                const date = new Date();
                it("should stringify", test(date, "new Date(" + date.getTime() + ")"));
            });
            describe("RegExp", () => {
                it("should stringify as shorthand", test(/[abc]/gi, "/[abc]/gi"));
            });
            describe("Number", () => {
                it("should stringify", test(new Number(10), "new Number(10)"));
            });
            describe("String", () => {
                it("should stringify", test(new String("abc"), "new String('abc')"));
            });
            describe("Boolean", () => {
                it("should stringify", test(new Boolean(true), "new Boolean(true)"));
            });
            describeIf("Buffer", typeof Buffer === "function", () => {
                it("should stringify", test(Buffer.from("test"), "new Buffer('test')"));
            });
            describeIf("BigInt", typeof BigInt === "function", () => {
                it("should stringify", test(BigInt("10"), "BigInt('10')"));
            });
            describe("Error", () => {
                it("should stringify", test(new Error("test"), "new Error('test')"));
            });
            describe("unknown native type", () => {
                it("should be omitted", test({
                    k: typeof process === "undefined"
                        ? window.navigator
                        : process
                }, "{}"));
            });
        });
        describeIf("ES6", typeof Array.from === "function", () => {
            describeIf("Map", typeof Map === "function", () => {
                it("should stringify", test(new Map([["key", "value"]]), "new Map([['key','value']])"));
            });
            describeIf("Set", typeof Set === "function", () => {
                it("should stringify", test(new Set(["key", "value"]), "new Set(['key','value'])"));
            });
            describe("arrow functions", () => {
                describe("should stringify", cases([
                    "(a, b) => a + b",
                    "o => { return o.a + o.b; }",
                    "(a, b) => { if (a) { return b; } }",
                    "(a, b) => ({ [a]: b })",
                    "a => b => () => a + b"
                ]));
                it("should reindent function bodies", test(evalValue("               () => {\n" +
                    "                 if (true) {\n" +
                    '                   return "hello";\n' +
                    "                 }\n" +
                    "               }"), '() => {\n  if (true) {\n    return "hello";\n  }\n}', 2));
                describeIf("arrows with patterns", isSupported("({x}) => x"), () => {
                    describe("should stringify", cases([
                        "({ x, y }) => x + y",
                        "({ x, y }) => { if (x === '}') { return y; } }",
                        "({ x, y = /[/})]/.test(x) }) => { return y ? x : 0; }"
                    ]));
                });
            });
            describe("generators", () => {
                it("should stringify", testRoundTrip("function* (x) { yield x; }"));
            });
            describe("class notation", () => {
                it("should stringify classes", testRoundTrip("class {}"));
                it("should stringify class and method", testRoundTrip("class { method() {} }"));
                it("should stringify with newline", testRoundTrip("class\n{ method() {} }"));
                it("should stringify with comment", testRoundTrip("class/*test*/\n{ method() {} }"));
            });
            describe("method notation", () => {
                it("should stringify", testRoundTrip("{a(b, c) { return b + c; }}"));
                it("should stringify generator methods", testRoundTrip("{*a(b) { yield b; }}"));
                describe("should not be fooled by tricky names", cases([
                    "{'function a'(b, c) { return b + c; }}",
                    "{'a(a'(b, c) { return b + c; }}",
                    "{'() => function '() {}}",
                    "{'['() { return x[y]()\n{ return true; }}}",
                    "{'() { return false;//'() { return true;\n}}"
                ]));
                it("should not be fooled by tricky generator names", testRoundTrip("{*'function a'(b, c) { return b + c; }}"));
                it("should not be fooled by empty names", testRoundTrip("{''(b, c) { return b + c; }}"));
                it("should not be fooled by keys that look like functions", () => {
                    const fn = evalValue('{ "() => ": () => () => 42 }')["() => "];
                    expect(index_1.stringify(fn)).toEqual("() => () => 42");
                });
                describe("should not be fooled by arrow functions", cases([
                    "{a:(b, c) => b + c}",
                    "{a:a => a + 1}",
                    "{'() => ':() => () => 42}",
                    '{\'() => "\':() => "() {//"}',
                    '{\'() => "\':() => "() {`//"}',
                    '{\'() => "\':() => "() {`${//"}',
                    '{\'() => "\':() => "() {/*//"}',
                    semver_1.satisfies(process.versions.node, "<=4 || >=10")
                        ? "{'a => function ':a => function () { return a + 1; }}"
                        : undefined
                ]));
                describe("should not be fooled by regexp literals", cases([
                    "{' '(s) { return /}/.test(s); }}",
                    "{' '(s) { return /abc/ .test(s); }}",
                    "{' '() { return x / y; // /}\n}}",
                    "{' '() { return / y; }//* } */}}",
                    "{' '() { return delete / y; }/.x}}",
                    "{' '() { switch (x) { case / y; }}/: }}}",
                    "{' '() { if (x) return; else / y;}/; }}",
                    "{' '() { return x in / y;}/; }}",
                    "{' '() { return x instanceof / y;}/; }}",
                    "{' '() { return new / y;}/.x; }}",
                    "{' '() { throw / y;}/.x; }}",
                    "{' '() { return typeof / y;}/; }}",
                    "{' '() { void / y;}/; }}",
                    "{' '() { return x, / y;}/; }}",
                    "{' '() { return x; / y;}/; }}",
                    "{' '() { return { x: / y;}/ }; }}",
                    "{' '() { return x + / y;}/.x; }}",
                    "{' '() { return x - / y;}/.x; }}",
                    "{' '() { return !/ y;}/; }}",
                    "{' '() { return ~/ y;}/.x; }}",
                    "{' '() { return x && / y;}/; }}",
                    "{' '() { return x || / y;}/; }}",
                    "{' '() { return x ^ / y;}/.x; }}",
                    "{' '() { return x * / y;}/.x; }}",
                    "{' '() { return x / / y;}/.x; }}",
                    "{' '() { return x % / y;}/.x; }}",
                    "{' '() { return x < / y;}/.x; }}",
                    "{' '() { return x > / y;}/.x; }}",
                    "{' '() { return x <= / y;}/.x; }}",
                    "{' '() { return x /= / y;}/.x; }}",
                    "{' '() { return x ? / y;}/ : false; }}"
                ]));
                describe("should not be fooled by computed names", () => {
                    it("1", test(evalValue('{ ["foobar".slice(3)](x) { return x + 1; } }'), "{bar(x) { return x + 1; }}"));
                    it("2", test(evalValue('{[((s,a,b)=>a+s(a)+","+s(b)+b)(JSON.stringify,"[((s,a,b)=>a+s(a)+\\",\\"+s(b)+b)(JSON.stringify,",")]() {}")]() {}}'), '{\'[((s,a,b)=>a+s(a)+","+s(b)+b)(JSON.stringify,"[((s,a,b)=>a+s(a)+\\\\",\\\\"+s(b)+b)(JSON.stringify,",")]() {}")]() {}\'() {}}'));
                    it("3", test(evalValue('{[`over${`6${"0".repeat(3)}`.replace("6", "9")}`]() { this.activateHair(); }}'), "{over9000() { this.activateHair(); }}"));
                    it("4", test(evalValue("{[\"() {'\"]() {''}}"), "{'() {\\''() {''}}"));
                    it("5", test(evalValue('{["() {`"]() {``}}'), "{'() {`'() {``}}"));
                    it("6", test(evalValue('{["() {/*"]() {/*`${()=>{/*}*/}}'), "{'() {/*'() {/*`${()=>{/*}*/}}"));
                });
                // These two cases demonstrate that branching on
                // METHOD_NAMES_ARE_QUOTED is unavoidable--you can't write code
                // without it that will pass both of these cases on both node.js 4
                // and node.js 10. (If you think you can, consider that the name and
                // toString of the first case when executed on node.js 10 are
                // identical to the name and toString of the second case when
                // executed on node.js 4, so good luck telling them apart without
                // knowing which node you're on.)
                describe("should handle different versions of node correctly", () => {
                    it("1", test(evalValue('{[((s,a,b)=>a+s(a)+","+s(b)+b)(JSON.stringify,"[((s,a,b)=>a+s(a)+\\",\\"+s(b)+b)(JSON.stringify,",")]() { return 0; /*")]() { return 0; /*() {/* */ return 1;}}'), '{\'[((s,a,b)=>a+s(a)+","+s(b)+b)(JSON.stringify,"[((s,a,b)=>a+s(a)+\\\\",\\\\"+s(b)+b)(JSON.stringify,",")]() { return 0; /*")]() { return 0; /*\'() { return 0; /*() {/* */ return 1;}}'));
                    it("2", test(evalValue('{\'[((s,a,b)=>a+s(a)+","+s(b)+b)(JSON.stringify,"[((s,a,b)=>a+s(a)+\\\\",\\\\"+s(b)+b)(JSON.stringify,",")]() { return 0; /*")]() { return 0; /*\'() {/* */ return 1;}}'), '{\'[((s,a,b)=>a+s(a)+","+s(b)+b)(JSON.stringify,"[((s,a,b)=>a+s(a)+\\\\",\\\\"+s(b)+b)(JSON.stringify,",")]() { return 0; /*")]() { return 0; /*\'() {/* */ return 1;}}'));
                });
                it("should not be fooled by comments", test(evalValue("{'method' /* a comment! */ () /* another comment! */ {}}"), "{method() /* another comment! */ {}}"));
                it("should stringify extracted methods", () => {
                    const fn = evalValue("{ foo(x) { return x + 1; } }").foo;
                    expect(index_1.stringify(fn)).toEqual("function foo(x) { return x + 1; }");
                });
                it("should stringify extracted generators", () => {
                    const fn = evalValue("{ *foo(x) { yield x; } }").foo;
                    expect(index_1.stringify(fn)).toEqual("function* foo(x) { yield x; }");
                });
                it("should stringify extracted methods with tricky names", () => {
                    const fn = evalValue('{ "a(a"(x) { return x + 1; } }')["a(a"];
                    expect(index_1.stringify(fn)).toEqual("function (x) { return x + 1; }");
                });
                it("should stringify extracted methods with arrow-like tricky names", () => {
                    const fn = evalValue('{ "() => function "(x) { return x + 1; } }')["() => function "];
                    expect(index_1.stringify(fn)).toEqual("function (x) { return x + 1; }");
                });
                it("should stringify extracted methods with empty names", () => {
                    const fn = evalValue('{ ""(x) { return x + 1; } }')[""];
                    expect(index_1.stringify(fn)).toEqual("function (x) { return x + 1; }");
                });
                it("should handle transplanted names", () => {
                    const fn = evalValue("{ foo(x) { return x + 1; } }").foo;
                    expect(index_1.stringify({ bar: fn })).toEqual("{bar:function foo(x) { return x + 1; }}");
                });
                it("should handle transplanted names with generators", () => {
                    const fn = evalValue("{ *foo(x) { yield x; } }").foo;
                    expect(index_1.stringify({ bar: fn })).toEqual("{bar:function* foo(x) { yield x; }}");
                });
                it("should reindent methods", test(evalValue("               {\n" +
                    "                 fn() {\n" +
                    "                   if (true) {\n" +
                    '                     return "hello";\n' +
                    "                   }\n" +
                    "                 }\n" +
                    "               }"), '{\n  fn() {\n    if (true) {\n      return "hello";\n    }\n  }\n}', 2));
            });
        });
        describe("ES2017", () => {
            describeIf("async functions", isSupported("(async function () {})"), () => {
                it("should stringify", testRoundTrip("async function (x) { await x; }"));
                it("should gracefully handle unexpected Function.toString formats", () => {
                    const origToString = Function.prototype.toString;
                    Function.prototype.toString = () => "{nope}";
                    try {
                        expect(index_1.stringify(evalValue("async function () {}"))).toEqual("void '{nope}'");
                    }
                    finally {
                        Function.prototype.toString = origToString;
                    }
                });
            });
            describeIf("async arrows", isSupported("async () => {}"), () => {
                describe("should stringify", cases([
                    "async (x) => x + 1",
                    "async x => x + 1",
                    "async x => { await x.then(y => y + 1); }"
                ]));
                describe("should stringify as object properties", cases([
                    "{f:async a => a + 1}",
                    semver_1.satisfies(process.versions.node, "<=4 || >=10")
                        ? "{'async a => function ':async a => function () { return a + 1; }}"
                        : undefined
                ]));
            });
        });
        describe("ES2018", () => {
            describeIf("async generators", isSupported("(async function* () {})"), () => {
                it("should stringify", testRoundTrip("async function* (x) { yield x; }"));
                it("should gracefully handle unexpected Function.toString formats", () => {
                    const origToString = Function.prototype.toString;
                    Function.prototype.toString = () => "{nope}";
                    try {
                        expect(index_1.stringify(evalValue("async function* () {}"))).toEqual("void '{nope}'");
                    }
                    finally {
                        Function.prototype.toString = origToString;
                    }
                });
            });
        });
        describe("global", () => {
            it("should access the global in the current environment", testRoundTrip("Function('return this')()"));
        });
    });
    describe("circular references", () => {
        it("should omit circular references", () => {
            const obj = { key: "value" };
            obj.obj = obj;
            const result = index_1.stringify(obj);
            expect(result).toEqual("{key:'value'}");
        });
        it("should restore value", () => {
            const obj = { key: "value" };
            obj.obj = obj;
            const result = index_1.stringify(obj, null, null, { references: true });
            expect(result).toEqual("(function(){var x={key:'value'};x.obj=x;return x;}())");
        });
        it("should omit recursive array value", () => {
            const obj = [1, 2, 3];
            obj.push(obj);
            const result = index_1.stringify(obj);
            expect(result).toEqual("[1,2,3,undefined]");
        });
        it("should restore array value", () => {
            const obj = [1, 2, 3];
            obj.push(obj);
            const result = index_1.stringify(obj, null, null, { references: true });
            expect(result).toEqual("(function(){var x=[1,2,3,undefined];x[3]=x;return x;}())");
        });
        it("should print repeated values when no references enabled", () => {
            const obj = {};
            const child = {};
            obj.a = child;
            obj.b = child;
            const result = index_1.stringify(obj);
            expect(result).toEqual("{a:{},b:{}}");
        });
        it("should restore repeated values", () => {
            const obj = {};
            const child = {};
            obj.a = child;
            obj.b = child;
            const result = index_1.stringify(obj, null, null, { references: true });
            expect(result).toEqual("(function(){var x={a:{}};x.b=x.a;return x;}())");
        });
        it("should restore repeated values with indentation", function () {
            const obj = {};
            const child = {};
            obj.a = child;
            obj.b = child;
            const result = index_1.stringify(obj, null, 2, { references: true });
            expect(result).toEqual("(function () {\nvar x = {\n  a: {}\n};\nx.b = x.a;\nreturn x;\n}())");
        });
    });
    describe("custom indent", () => {
        it("string", () => {
            const result = index_1.stringify({
                test: [1, 2, 3],
                nested: {
                    key: "value"
                }
            }, null, "\t");
            expect(result).toEqual("{\n" +
                "\ttest: [\n\t\t1,\n\t\t2,\n\t\t3\n\t],\n" +
                "\tnested: {\n\t\tkey: 'value'\n\t}\n" +
                "}");
        });
        it("integer", () => {
            const result = index_1.stringify({
                test: [1, 2, 3],
                nested: {
                    key: "value"
                }
            }, null, 2);
            expect(result).toEqual("{\n" +
                "  test: [\n    1,\n    2,\n    3\n  ],\n" +
                "  nested: {\n    key: 'value'\n  }\n" +
                "}");
        });
        it("float", () => {
            const result = index_1.stringify({
                test: [1, 2, 3],
                nested: {
                    key: "value"
                }
            }, null, 2.6);
            expect(result).toEqual("{\n" +
                "  test: [\n    1,\n    2,\n    3\n  ],\n" +
                "  nested: {\n    key: 'value'\n  }\n" +
                "}");
        });
    });
    describe("replacer function", () => {
        it("should allow custom replacements", () => {
            let callCount = 0;
            const result = index_1.stringify({
                test: "value"
            }, function (value, indent, next) {
                callCount++;
                if (typeof value === "string") {
                    return '"hello"';
                }
                return next(value);
            });
            expect(callCount).toEqual(2);
            expect(result).toEqual('{test:"hello"}');
        });
        it("change primitive to object", () => {
            const result = index_1.stringify({
                test: 10
            }, function (value, indent, next) {
                if (typeof value === "number") {
                    return next({ obj: "value" });
                }
                return next(value);
            });
            expect(result).toEqual("{test:{obj:'value'}}");
        });
        it("change object to primitive", () => {
            const result = index_1.stringify({
                test: 10
            }, value => Object.prototype.toString.call(value));
            expect(result).toEqual("[object Object]");
        });
        it("should support object functions", () => {
            function makeRaw(str) {
                const fn = () => {
                    /* Noop. */
                };
                fn.__expression = str;
                return fn;
            }
            const result = index_1.stringify({
                "no-console": makeRaw(`process.env.NODE_ENV === 'production' ? 'error' : 'off'`),
                "no-debugger": makeRaw(`process.env.NODE_ENV === 'production' ? 'error' : 'off'`)
            }, (val, indent, stringify) => {
                if (val && val.__expression) {
                    return val.__expression;
                }
                return stringify(val);
            }, 2);
            expect(result).toEqual(`{
  'no-console': process.env.NODE_ENV === 'production' ? 'error' : 'off',
  'no-debugger': process.env.NODE_ENV === 'production' ? 'error' : 'off'
}`);
        });
    });
    describe("max depth", () => {
        const obj = { a: { b: { c: 1 } } };
        it("should get all object", test(obj, "{a:{b:{c:1}}}"));
        it("should get part of the object", test(obj, "{a:{b:{}}}", null, { maxDepth: 2 }));
        it("should get part of the object when tracking references", test(obj, "{a:{b:{}}}", null, { maxDepth: 2, references: true }));
    });
    describe("property based", () => {
        it("should produce string evaluating to the original value", () => {
            fc.assert(fc.property(fc.anything(), value => {
                expect(evalValue(index_1.stringify(value))).toEqual(value);
            }));
        });
    });
});
//# sourceMappingURL=index.spec.js.map