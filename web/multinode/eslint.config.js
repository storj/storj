// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

import globals from 'globals';
import js from '@eslint/js';
import pluginVue from 'eslint-plugin-vue';
import importPlugin from 'eslint-plugin-import';
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
        ],
    },
    js.configs.recommended,
    importPlugin.flatConfigs.recommended,
    pluginVue.configs['flat/recommended'],
    vueTsConfigs.recommended,
    {
        languageOptions: {
            globals: globals.node,
            parser: vueEsLintParser,
            parserOptions: {
                parser: '@typescript-eslint/parser',
                sourceType: 'module',
                ecmaVersion: 2020,
            },
        },
        plugins: { '@stylistic': stylistic },
        rules: {
            'no-console': process.env.NODE_ENV === 'production' ? 'warn' : 'off',
            'no-debugger': process.env.NODE_ENV === 'production' ? 'warn' : 'off',
            'vue/html-indent': ['warn', 4],
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

            '@typescript-eslint/no-unused-vars': ['warn', { 'argsIgnorePattern': '^_' }],
            '@typescript-eslint/no-empty-function': 'off',
            '@typescript-eslint/no-var-requires': 'off',

            // TypeScript compilation already ensures that named imports exist in the referenced module
            'import/named': 'off',
            'import/order': ['error', {
                'pathGroups': [
                    {
                        'group': 'external',
                        'pattern': 'vue-property-decorator',
                        'position': 'before',
                    },
                    {
                        'group': 'internal',
                        'pattern': '@/app/{components,views,layouts}/**',
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
            'eqeqeq': ['error'],

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
    },
]);
