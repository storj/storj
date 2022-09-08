// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

module.exports = {
    'env': {
        'es2020': true,
        'node': true,
        'jest': true,
    },
    'plugins': [
        'stylelint-scss',
    ],
    'extends': 'stylelint-config-standard-vue/scss',
    'customSyntax': 'postcss-html',
    'rules': {
        'indentation': 4,
        'string-quotes': 'single',
        'no-duplicate-selectors': true,
        'no-descending-specificity': null,
        'selector-max-attribute': 1,
        'selector-combinator-space-after': 'always',
        'selector-attribute-operator-space-before': 'never',
        'selector-attribute-operator-space-after': 'never',
        'selector-attribute-brackets-space-inside': 'never',
        'declaration-block-trailing-semicolon': 'always',
        'declaration-colon-space-before': 'never',
        'declaration-colon-space-after': 'always-single-line',
        'number-leading-zero': 'always',
        'function-url-quotes': 'always',
        'font-family-name-quotes': 'always-unless-keyword',
        'comment-whitespace-inside': 'always',
        'comment-empty-line-before': 'always',
        'rule-empty-line-before': 'always-multi-line',
        'selector-pseudo-element-colon-notation': 'single',
        'selector-pseudo-class-parentheses-space-inside': 'never',
        'selector-max-type': 3,
        'font-family-no-missing-generic-family-keyword': true,
        'at-rule-no-unknown': null,
        'scss/at-rule-no-unknown': true,
        'media-feature-range-operator-space-before': 'always',
        'media-feature-range-operator-space-after': 'always',
        'media-feature-parentheses-space-inside': 'never',
        'media-feature-colon-space-before': 'never',
        'media-feature-colon-space-after': 'always',
        'selector-pseudo-element-no-unknown': [
            true,
            {
                'ignorePseudoElements': ['v-deep'],
            },
        ],
        'selector-class-pattern': '.*',
        'custom-property-pattern': '.*',
    },
};
