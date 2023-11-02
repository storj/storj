// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

/**
 * plugins/vuetify.ts
 *
 * Framework documentation: https://vuetifyjs.com`
 */

import 'vuetify/styles';
import '@mdi/font/css/materialdesignicons.css';
import '@fontsource-variable/inter';
import { createVuetify } from 'vuetify';
import { md3 } from 'vuetify/blueprints';

import '../styles/styles.scss';

// https://vuetifyjs.com/en/introduction/why-vuetify/#feature-guides
export default createVuetify({
    // Use blueprint for Material Design 3
    blueprint: md3,
    theme: {
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
                    green: '#00e366',
                    purple2: '#A18EFF',
                },
            },
        },
    },
    defaults: {
        global: {
            // ripple: false,
        },
        VDataTable: {
            fixedHeader: true,
            noDataText: 'Results not found',
        },
        VBtn: {
            density: 'default',
            rounded: 'lg',
            class: 'text-none font-weight-bold',
            style: 'letter-spacing:0;',
        },
        VTooltip: {
            transition: 'fade-transition',
        },
        VSelect: {
            // rounded: 'lg',
            variant: 'outlined',
            color: 'secondary',
        },
        VTextField: {
            rounded: 'lg',
            variant: 'outlined',
            color: 'secondary',
        },
        VList: {
            rounded: 'lg',
        },
        VListItem: {
            rounded: 'lg',
        },
        VListItemTitle: {
            class: 'text-body-2 font-weight-medium',
        },
        VCard: {
            border: true,
            rounded: 'xlg',
        },
        VTable: {
            class: 'elevation-0',
        },
        VCheckbox: {
            color: 'primary',
        },
        VAlert: {
            rounded: 'xlg',
        },
        VChip: {
            rounded: 'lg',
        },
        // VTable: {
        //     elevation: 0,
        // },
    },
});