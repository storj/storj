// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

import { Plugin } from 'vite';
import { build } from 'esbuild';
import { createVuetify } from 'vuetify';

import { THEME_OPTIONS } from '../../vuetify-poc/src/plugins/theme';

export default function vuetifyThemeCSS(): Plugin {
    const name = 'vuetify-theme-css';
    const virtualModuleId = 'virtual:' + name;
    const resolvedVirtualModuleId = '\0' + virtualModuleId;

    const theme = createVuetify({ theme: THEME_OPTIONS }).theme;
    const themeURLs: Record<string, string> = {};

    return {
        name,

        async buildStart() {
            for (const name of Object.keys(theme.themes.value)) {
                theme.global.name.value = name;

                const result = await build({
                    stdin: {
                        contents: theme.styles.value,
                        loader: 'css',
                    },
                    write: false,
                    minify: true,
                });

                const refId = this.emitFile({
                    type: 'asset',
                    name: `theme-${name}.css`,
                    source: result.outputFiles[0].text,
                });
                themeURLs[name] = `__VITE_ASSET__${refId}__`;
            }
        },

        resolveId(id: string) {
            if (id === virtualModuleId) return resolvedVirtualModuleId;
        },

        load(id: string) {
            if (id === resolvedVirtualModuleId) {
                return `export const themeURLs = {${
                    Object.entries(themeURLs)
                        .map(([name, url]) => `'${name}':'${url}'`)
                        .join(',')
                }};`;
            }
        },
    };
}
