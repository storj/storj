define(["require", "exports"], function (require, exports) {
    "use strict";
    Object.defineProperty(exports, "__esModule", { value: true });
    exports.id = void 0;
    var idCounter = 0;
    function id() {
        return idCounter++;
    }
    exports.id = id;
});
