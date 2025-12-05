// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

import 'vuetify/styles';
import '@fontsource-variable/inter';
import { createVuetify } from 'vuetify';
import { md3 } from 'vuetify/blueprints';
import { aliases, mdi } from 'vuetify/iconsets/mdi-svg';
import '@/styles/styles.scss';

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

export default createVuetify({
    blueprint: md3,
    theme: THEME_OPTIONS,
    icons: {
        defaultSet: 'mdi',
        aliases,
        sets: {
            mdi,
        },
    },
    defaults: {
        global: {
            // ripple: false,
        },
        VDataTable: {
            fixedHeader: true,
            noDataText: 'Results not found',
            hover: true,
            rounded: 'lg',
        },
        VBtn: {
            density: 'default',
            rounded: 'md',
            class: 'text-none font-weight-bold',
            style: 'letter-spacing:0;',
        },
        VTooltip: {
            location: 'top',
            transition: 'fade-transition',
            rounded: 'lg',
        },
        VSelect: {
            rounded: 'md',
            variant: 'outlined',
            color: 'secondary',
            menuProps: { rounded: 'lg' },
        },
        VTextField: {
            rounded: 'md',
            variant: 'outlined',
            color: 'secondary',
        },
        VTextarea: {
            rounded: 'md',
            variant: 'outlined',
            color: 'secondary',
        },
        VAutoComplete: {
            color: 'secondary',
        },
        VList: {
            rounded: 'lg',
        },
        VListItem: {
            rounded: 'md',
        },
        VListItemTitle: {
            class: 'text-body-2 font-weight-medium',
        },
        VCard: {
            border: true,
            rounded: 'lg',
        },
        VTable: {
            class: 'elevation-1 rounded-md',
            hover: true,
        },
        VCheckbox: {
            color: 'primary',
            class: 'ml-n1',
            hideDetails: 'auto',
        },
        VAlert: {
            rounded: 'lg',
        },
        VChip: {
            rounded: 'md',
        },
        VChipGroup: {
            color: 'primary',
            variant: 'outlined',
        },
        VRadio: {
            color: 'primary',
            hideDetails: 'auto',
        },
        VSwitch: {
            color: 'primary',
            hideDetails: 'auto',
            inset: true,
        },
        VPagination: {
            rounded: 'lg',
            density: 'comfortable',
            activeColor: 'primary',
        },
        VMenu: {
            rounded: 'lg',
            transition: 'fade-transition',
        },
        VDialog: {
            rounded: 'lg',
            persistent: true,
            scrollable: true,
            transition: 'fade-transition',
        },
    },
});
