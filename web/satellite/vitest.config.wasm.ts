// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

import { defineConfig } from 'vitest/config';

export default defineConfig({
    test: {
        globals: true,
        environment: 'node',
        setupFiles: ['./vitest.setup.wasm.ts'],
        include: ['tests/wasm/**/*.spec.ts'],
    },
});
