// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

import globals from 'globals';
import js from '@eslint/js';
import pluginVue from 'eslint-plugin-vue';
import { flatConfigs } from 'eslint-plugin-import-x';
import stylistic from '@stylistic/eslint-plugin';
import { defineConfigWithVueTs, vueTsConfigs } from '@vue/eslint-config-typescript';
import vueEsLintParser from 'vue-eslint-parser';

export default defineConfigWithVueTs([
    {
        ignores: [
            'dist',
            'node_modules',
            'coverage',
            'tests/unit/ignore',
            'static/wasm',
            'src/utils/accessGrant.worker.js',
            'wasm',
            'scripts/static',
            'src/api/v1.gen.ts',
        ],
    },
    js.configs.recommended,
    flatConfigs.recommended,
    pluginVue.configs['flat/recommended'],
    vueTsConfigs.recommended,
    {
        languageOptions: {
            globals: globals.node,
            parser: vueEsLintParser,
            parserOptions: {
                parser: '@typescript-eslint/parser',
                sourceType: 'module',
                ecmaVersion: 'latest',
            },
        },
        plugins: { '@stylistic': stylistic },
        rules: {
            'no-console': process.env.NODE_ENV === 'production' ? 'warn' : 'off',
            'no-debugger': process.env.NODE_ENV === 'production' ? 'warn' : 'off',

            'vue/html-indent': ['warn', 4],

            '@stylistic/object-curly-newline': ['error', {
                'multiline': true,
                'consistent': true,
            }],
            '@stylistic/indent': ['warn', 4, { 'SwitchCase': 0 }],
            '@stylistic/object-curly-spacing': ['error', 'always'],
            '@stylistic/comma-dangle': ['error', 'always-multiline'],
            '@stylistic/arrow-spacing': ['error'],
            '@stylistic/space-in-parens': ['error'],
            '@stylistic/space-before-blocks': ['error'],
            '@stylistic/keyword-spacing': ['error'],
            '@stylistic/no-trailing-spaces': ['error'],
            '@stylistic/linebreak-style': ['error', 'unix'],
            '@stylistic/no-multiple-empty-lines': ['error', { 'max': 1 }],
            '@stylistic/no-tabs': 'warn',
            '@stylistic/quotes': ['error', 'single', { 'allowTemplateLiterals': 'always' }],
            '@stylistic/semi': ['error', 'always'],

            '@typescript-eslint/consistent-type-imports': [
                'error',
                {
                    prefer: 'type-imports',
                    fixStyle: 'inline-type-imports',
                },
            ],
            '@typescript-eslint/no-unused-vars': ['warn', {
                'argsIgnorePattern': '^_',
                'varsIgnorePattern': '^_',
            }],
            '@typescript-eslint/no-empty-function': 'off',
            '@typescript-eslint/no-var-requires': 'off',

            // TypeScript compilation already ensures that named imports exist in the referenced module
            'import-x/named': 'off',
            'import-x/order': ['error', {
                'pathGroups': [
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
            'import-x/default': 'off',
            'import-x/no-unresolved': ['error', { ignore: ['^virtual:'] }],
            'import-x/no-duplicates': ['error', { 'prefer-inline': true }],

            'no-duplicate-imports': 'off',

            'eqeqeq': ['error'],

            // Prevent Vuetify v3 typography class names from being reintroduced after the v4 migration.
            // v4 equivalents: text-h1→text-display-large, text-h2→text-display-medium, text-h3→text-display-small,
            // text-h4→text-headline-large, text-h5→text-headline-medium, text-h6→text-title-large,
            // text-subtitle-1→text-title-medium, text-subtitle-2→text-title-small,
            // text-body-1→text-body-large, text-body-2→text-body-medium,
            // text-caption→text-body-small, text-overline→text-label-medium
            // See: https://vuetifyjs.com/en/styles/text-and-typography/
            'vue/no-restricted-class': ['error',
                'text-h1', 'text-h2', 'text-h3', 'text-h4', 'text-h5', 'text-h6',
                'text-subtitle-1', 'text-subtitle-2',
                'text-body-1', 'text-body-2',
                'text-caption', 'text-overline',
            ],

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
            'import-x/resolver': {
                typescript: {
                    alwaysTryTypes: true,
                    project: './tsconfig.json',
                },
                node: true,
            },
            'import-x/parsers': {
                '@typescript-eslint/parser': ['.ts'],
                'vue-eslint-parser': ['.vue'],
            },
        },
    },
]);
