// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

const vueJest = require('vue-jest/lib/template-compiler');

module.exports = {
    process(content) {
        const { render } = vueJest({
            content,
            attrs: {
                functional: false,
            },
        });

        return `module.exports = { render: ${render} }`;
    },
};