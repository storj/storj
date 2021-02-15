(function (factory) {
    if (typeof module === "object" && typeof module.exports === "object") {
        var v = factory(require, exports);
        if (v !== undefined) module.exports = v;
    }
    else if (typeof define === "function" && define.amd) {
        define(["require", "exports", "parse5"], factory);
    }
})(function (require, exports) {
    "use strict";
    Object.defineProperty(exports, "__esModule", { value: true });
    var parse5 = require("parse5");
    /**
     * Parse a vue file's contents.
     */
    function parse(input, tag, options) {
        var emptyExport = options && options.emptyExport !== undefined ? options.emptyExport : true;
        var node = getNode(input, tag, options);
        var parsed = padContent(node, input);
        // Add a default export of empty object if target tag script not found.
        // This fixes a TypeScript issue of "not a module".
        if (!parsed && tag === 'script' && emptyExport) {
            parsed = '// tslint:disable\nimport Vue from \'vue\'\nexport default Vue\n';
        }
        return parsed;
    }
    exports.parse = parse;
    /**
     * Pad the space above node with slashes (preserves content line/col positions in a file).
     */
    function padContent(node, input) {
        if (!node || !node.__location)
            return '';
        var nodeContent = input.substring(node.__location.startTag.endOffset, node.__location.endTag.startOffset);
        var preNodeContent = input.substring(0, node.__location.startTag.endOffset);
        var nodeLines = (preNodeContent.match(new RegExp('\n', 'g')) || []).length + 1;
        var remainingSlashes = preNodeContent.replace(/[\s\S]/gi, '/');
        var nodePrePad = '';
        var nodePad = '';
        // Reserve space for tslint:disable (if possible).
        if (nodeLines > 2) {
            nodePrePad = '//' + '\n';
            nodeLines--;
            remainingSlashes = remainingSlashes.substring(3);
        }
        // Pad with slashes (comments).
        for (var i = 1; i < nodeLines; i++) {
            nodePad += '//' + '\n';
            remainingSlashes = remainingSlashes.substring(3);
        }
        // Add tslint:disable and tslint:enable (if possible).
        if (nodePrePad && remainingSlashes.length > 50) {
            nodePrePad = '// tslint:disable' + '\n';
            remainingSlashes = remainingSlashes.substring('// tslint:disable\n// tslint:enable'.length);
            remainingSlashes = remainingSlashes.replace(/[\s\S]/gi, ' ') + '   // tslint:enable';
        }
        return nodePrePad + nodePad + remainingSlashes + nodeContent;
    }
    /**
     * Get an array of all the nodes (tags).
     */
    function getNodes(input) {
        var rootNode = parse5.parseFragment(input, { locationInfo: true });
        return rootNode.childNodes;
    }
    exports.getNodes = getNodes;
    /**
     * Get the node.
     */
    function getNode(input, tag, options) {
        // Set defaults.
        var lang = options ? options.lang : undefined;
        // Parse the Vue file nodes (tags) and find a match.
        return getNodes(input).find(function (node) {
            var tagFound = tag === node.nodeName;
            var tagHasAttrs = ('attrs' in node);
            var langEmpty = lang === undefined;
            var langMatch = false;
            if (lang) {
                langMatch = tagHasAttrs && node.attrs.find(function (attr) {
                    return attr.name === 'lang' && Array.isArray(lang)
                        ? lang.indexOf(attr.value) !== -1
                        : attr.value === lang;
                }) !== undefined;
            }
            return tagFound && (langEmpty || langMatch);
        });
    }
    exports.getNode = getNode;
});
