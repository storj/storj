// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

module.exports = {
    root: true,
    env: {
        node: true
    },
    extends: [
        'plugin:vue/recommended',
        'eslint:recommended',
        '@vue/typescript/recommended',
    ],
    parserOptions: {
        ecmaVersion: 2020
    },
    plugins: ["storj"],
    rules: {
        "linebreak-style": ["error", "unix"],

        'no-console': process.env.NODE_ENV === 'production' ? 'warn' : 'off',
        'no-debugger': process.env.NODE_ENV === 'production' ? 'warn' : 'off',

        "indent": ["warn", 4],
        "vue/html-indent": ["warn", 4],

        "@typescript-eslint/no-unused-vars": [
            "warn", {
                "vars": "all",
                "args": "all",
                "argsIgnorePattern": "^_"
            }],

        '@typescript-eslint/no-empty-function': "off",
        '@typescript-eslint/no-var-requires': "off",

        "vue/multi-word-component-names": ["off"],
        "vue/max-attributes-per-line": ["off"],
        "vue/singleline-html-element-content-newline": ["off"],

        "vue/block-lang": ["error", {"script": {"lang": "ts"}}],
        "vue/html-button-has-type": ["error"],
        "vue/no-unused-properties": ["warn"],
        "vue/no-unused-refs": ["warn"],
        "vue/no-useless-v-bind": ["warn"],

        'vue/no-unregistered-components': ['warn', { ignorePatterns: ['router-link', 'router-view'] }],

        'storj/vue/require-annotation': 'warn',
    },
}