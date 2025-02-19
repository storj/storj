// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

import { createVuetify } from 'vuetify';

type ThemeOptions = NonNullable<NonNullable<Parameters<typeof createVuetify>[0]>['theme']>;

export const THEME_OPTIONS: ThemeOptions = {
    cspNonce: 'dQw4w9WgXcQ',
    themes: {
        light: {
            dark: false,
            colors: {
                primary: '#0052FF',
                secondary: '#091C45',
                background: '#fcfcfd',
                surface: '#FFF',
                info: '#0059D0',
                help: '#FFA800',
                success: '#00B661',
                warning: '#FF7F00',
                error: '#FF0149',
                purple: '#7B61FF',
                purple2: '#502EFF',
                blue7: '#090920',
                blue6:  '#091c45',
                blue5: '#0218A7',
                blue4: '#0059D0',
                blue2: '#003ACD',
                yellow: '#FFC600',
                yellow2: '#FFB018',
                orange: '#FFA800',
                green: '#00E366',
                paragraph: '#283968',
            },
        },
        dark: {
            dark: true,
            colors: {
                primary: '#0052FF',
                secondary: '#537CFF',
                background: '#000a20',
                surface: '#000b21',
                success: '#00E366',
                help: '#FFC600',
                error: '#FF0149',
                purple: '#A18EFF',
                purple2: '#A18EFF',
                blue7: '#090920',
                blue6:  '#091c45',
                blue5: '#2196f3',
                blue4: '#0059D0',
                blue2: '#003ACD',
                yellow: '#FFC600',
                yellow2: '#FFB018',
                orange: '#FFA800',
                warning: '#FF8A00',
                green: '#00E366',
            },
        },
    },
};
