// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

module.exports = {
    root: true,
    env: {
        node: true,
    },
    extends: [
        'plugin:vue/recommended',
        'eslint:recommended',
        '@vue/typescript/recommended',
        'plugin:import/recommended',
        'plugin:import/typescript',
    ],
    parser: 'vue-eslint-parser',
    parserOptions: {
        parser: '@typescript-eslint/parser',
        sourceType: 'module',
        ecmaVersion: 2020,
    },
    plugins: ['eslint-plugin-import'],
    rules: {
        'linebreak-style': ['error', 'unix'],

        'no-console': process.env.NODE_ENV === 'production' ? 'warn' : 'off',
        'no-debugger': process.env.NODE_ENV === 'production' ? 'warn' : 'off',

        'no-tabs': 'warn',
        'indent': ['warn', 4],
        'vue/html-indent': ['warn', 4],
        '@typescript-eslint/indent': ['warn', 4, {"SwitchCase": 0}],

        '@typescript-eslint/no-unused-vars': 'off',
        '@typescript-eslint/no-empty-function': 'off',
        '@typescript-eslint/no-var-requires': 'off',

        'no-multiple-empty-lines': ['error', { 'max': 1 }],

        'import/order': ['error', {
            'pathGroups': [
                {
                    'group': 'external',
                    'pattern': 'vue-property-decorator',
                    'position': 'before',
                },
                {
                    'group': 'internal',
                    'pattern': '@/{components,views,layouts}/**',
                    'position': 'after',
                },
                {
                    'group': 'internal',
                    'pattern': '@/../static/**',
                    'position': 'after',
                },
                {
                    'group': 'internal',
                    'pattern': '@/assets/**',
                    'position': 'after',
                },
            ],
            'newlines-between': 'always',
        }],
        'import/no-unresolved': ['error', { ignore: ['^virtual:'] }],
        'no-duplicate-imports': 'error',
        'import/default': 'off',
        'vue/script-setup-uses-vars': 'error',
        'object-curly-spacing': ['error', 'always'],
        'quotes': ['error', 'single', { 'allowTemplateLiterals': true }],
        'semi': ['error', 'always'],
        'keyword-spacing': ['error'],
        'comma-dangle': ['error', 'always-multiline'],
        'no-trailing-spaces': ['error'],
        'eqeqeq': ['error'],
        'comma-spacing': ['error'],
        'arrow-spacing': ['error'],
        'space-in-parens': ['error'],
        'space-before-blocks': ['error'],

        'vue/multi-word-component-names': ['off'],
        'vue/max-attributes-per-line': ['off'],
        'vue/singleline-html-element-content-newline': ['off'],

        'vue/block-lang': ['error', { 'script': { 'lang': 'ts' } }],
        'vue/html-button-has-type': ['error'],
        'vue/no-unused-properties': ['warn'],
        'vue/no-unused-refs': ['warn'],
        'vue/no-unused-vars': ['warn'],
        'vue/no-useless-v-bind': ['warn'],
        'vue/no-v-model-argument': ['off'],
        'vue/valid-v-slot': ['error', { 'allowModifiers': true }],

        'vue/no-useless-template-attributes': ['off'], // TODO: fix later
        'vue/no-multiple-template-root': ['off'], // it's possible to have multiple roots in template in Vue 3

        'vue/no-undef-components': ['warn', { ignorePatterns: ['router-link', 'router-view'] }],

        'vue/no-v-html': ['error'],
    },
    settings: {
        'import/resolver': {
            'eslint-import-resolver-custom-alias': {
                'alias': {
                    '@': './src',
                },
                extensions: ['.ts', '.spec.ts', '.vue'],
            },
            typescript: {
                alwaysTryTypes: true,
                project: './tsconfig.json',
            },
            node: true,
        },
        'import/parsers': {
            '@typescript-eslint/parser': ['.ts'],
            'vue-eslint-parser': ['.vue'],
        },
    },
};
