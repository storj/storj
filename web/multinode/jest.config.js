// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

module.exports = {
    preset: '@vue/cli-plugin-unit-jest/presets/typescript',
    setupFiles: ['./jest.setup.ts'],
    testEnvironment: 'jsdom',
    transform: {
        '^.+\\.svg$': '<rootDir>/tests/unit/mock/svgTransform.js',
    },
    transformIgnorePatterns: ['/node_modules/(?!(vuetify|@mdi/font)/)'],
};