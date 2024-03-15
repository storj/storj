// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

import { createVuetify } from 'vuetify';

type ThemeOptions = NonNullable<NonNullable<Parameters<typeof createVuetify>[0]>['theme']>;

export const THEME_OPTIONS: ThemeOptions = {
    themes: {
        light: {
            colors: {
                primary: '#0052FF',
                secondary: '#091C45',
                background: '#FFF',
                surface: '#FFF',
                info: '#0059D0',
                help: '#FFA800',
                success: '#00AC26',
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
            colors: {
                primary: '#0052FF',
                secondary: '#537CFF',
                background: '#090927',
                success: '#00AC26',
                help: '#FFC600',
                error: '#FF0149',
                surface: '#090927',
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
