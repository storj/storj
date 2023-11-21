// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

import { Plugin } from 'vite';
import { build } from 'esbuild';

export default function papaParseWorker(): Plugin {
    const name = 'papa-parse-worker';
    const virtualModuleId = 'virtual:' + name;
    const resolvedVirtualModuleId = '\0' + virtualModuleId;

    let refId = '';
    let workerCode = '';

    return {
        name,

        async buildStart() {
            if (!workerCode) {
                // Trick Papa Parse into thinking it's being imported by RequireJS
                // so we can capture the AMD callback.
                let factory: (() => unknown) | undefined;
                global.define = (_: unknown, callback: () => void) => {
                    factory = callback;
                };
                global.define.amd = true;
                await import('papaparse');
                delete global.define;

                if (!factory) {
                    throw new Error('Failed to capture Papa Parse AMD callback');
                }

                workerCode = `
                    var global = (function() {
                        if (typeof self !== 'undefined') { return self; }
                        if (typeof window !== 'undefined') { return window; }
                        if (typeof global !== 'undefined') { return global; }
                        return {};
                    })();
                    global.IS_PAPA_WORKER = true;
                    (${factory.toString()})();
                `;

                const result = await build({
                    stdin: {
                        contents: workerCode,
                    },
                    write: false,
                    minify: true,
                });

                workerCode = result.outputFiles[0].text;
            }

            refId = this.emitFile({
                type: 'asset',
                name: 'papaparse-worker.js',
                source: workerCode,
            });
        },

        resolveId(id: string) {
            if (id === virtualModuleId) return resolvedVirtualModuleId;
        },

        load(id: string) {
            if (id === resolvedVirtualModuleId) {
                return `export default '__VITE_ASSET__${refId}__';`;
            }
        },
    };
}
