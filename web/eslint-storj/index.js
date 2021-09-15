// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

module.exports = {
    rules: {
        "vue/require-annotation": {
            meta: {
                fixable: "code",
            },
            create: function(context) {
                return {
                    Decorator(node) {
                        let isComponent = false;
                        const expr = node.expression;
                        if(expr.name === "Component"){
                            isComponent = true;
                        } else if (expr.callee && expr.callee.name === "Component"){
                            isComponent = true;
                        }
                        if(!isComponent){ return; }

                        const commentsBefore = context.getCommentsBefore(node);
                        const decoratorLine = node.loc.start.line;
                        let annotated = false;
                        commentsBefore.forEach(comment => {
                            if(comment.loc.start.line === decoratorLine - 1){
                                if(comment.value.trim() === "@vue/component") {
                                    annotated = true;
                                }
                            }
                        })
                        if(!annotated){
                            context.report({
                                node: node,
                                message: '@Component requires // @vue/component',
                                fix: function(fixer) {
                                    return fixer.insertTextBefore(node, "// @vue/component\n");
                                }
                            });
                        }
                    }
                };
            }
        }
    }
};