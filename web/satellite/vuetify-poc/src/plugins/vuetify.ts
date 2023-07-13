// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

/**
 * plugins/vuetify.ts
 *
 * Framework documentation: https://vuetifyjs.com`
 */

import '@mdi/font/css/materialdesignicons.css';
import { createVuetify } from 'vuetify';
import { VDataTable } from 'vuetify/labs/VDataTable';
import { md3 } from 'vuetify/blueprints';

import '../styles/styles.scss';

// https://vuetifyjs.com/en/introduction/why-vuetify/#feature-guides
export default createVuetify({
    // Use blueprint for Material Design 2
    // blueprint: md2,
    // Use blueprint for Material Design 3
    blueprint: md3,
    theme: {
        themes: {
            light: {
                colors: {
                    primary: '#0149FF',
                    secondary: '#0218A7',
                    info: '#537CFF',
                    success: '#00AC26',
                    warning: '#FF8A00',
                    error: '#FF458B',
                    error2: '#FF0149',
                    surface: '#FFF',
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
                    // background: '#010923',
                    // background: '#0c121d',
                    background: '#0d1116',
                    error: '#FF458B',
                    error2: '#FF0149',
                    // surface: '#010923',
                    // surface: '#0c121d', dark bluish
                    surface: '#0d1116',
                    purple: '#7B61FF',
                    blue6:  '#091c45',
                    blue5: '#2196f3',
                    blue4: '#0059D0',
                    blue2: '#003ACD',
                    yellow: '#FFC600',
                    yellow2: '#FFB701',
                    orange: '#FFA800',
                    warning: '#FF8A00',
                    green: '#00B150',
                    purple2: '#7B61FF',
                },
            },
        },
    },
    components: {
        VDataTable,
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
            // elevation: 1,
            density: 'default',
            // height: 48,
            rounded: 'lg',
            // textTransform: 'none',
            class: 'text-capitalize font-weight-bold',
            style: 'letter-spacing:0;',
        },
        VTooltip: {
            transition: 'fade-transition',
        },
    },
});