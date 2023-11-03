// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

import { createVuetify } from 'vuetify';

type ThemeOptions = NonNullable<NonNullable<Parameters<typeof createVuetify>[0]>['theme']>;

export const THEME_OPTIONS: ThemeOptions = {
    themes: {
        light: {
            colors: {
                primary: '#0149FF',
                secondary: '#0218A7',
                background: '#FFF',
                surface: '#FFF',
                info: '#0059D0',
                help: '#FFA800',
                success: '#00AC26',
                warning: '#FF7F00',
                error: '#FF0149',
                purple: '#7B61FF',
                blue6:  '#091c45',
                blue5: '#0218A7',
                blue4: '#0059D0',
                blue2: '#003ACD',
                yellow: '#FFC600',
                yellow2: '#FFB701',
                orange: '#FFA800',
                green: '#00B150',
                purple2: '#502EFF',
            },
        },
        dark: {
            colors: {
                primary: '#0149FF',
                secondary: '#537CFF',
                background: '#090920',
                success: '#00AC26',
                help: '#FFC600',
                error: '#FF0149',
                surface: '#090920',
                purple: '#A18EFF',
                blue6:  '#091c45',
                blue5: '#2196f3',
                blue4: '#0059D0',
                blue2: '#003ACD',
                yellow: '#FFC600',
                yellow2: '#FFB701',
                orange: '#FFA800',
                warning: '#FF8A00',
                // green: '#00B150',
                green: '#00e366',
                purple2: '#A18EFF',
            },
        },
    },
};
