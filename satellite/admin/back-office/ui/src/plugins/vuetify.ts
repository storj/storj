// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

/**
 * plugins/vuetify.ts
 *
 * Framework documentation: https://vuetifyjs.com`
 */

import '@mdi/font/css/materialdesignicons.css';
import 'vuetify/styles';
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
                    background: '#F9F9F9',
                    surface: '#FFF',
                    info: '#537CFF',
                    success: '#00AC26',
                    warning: '#FF8A00',
                    error: '#FF0149',
                    pink: '#FF458B',
                    purple: '#502EFF',
                    yellow: '#FFC600',
                    orange: '#FFA800',
                    green: '#00B150',
                    grey: '#F1F1F1',
                },
            },
            dark: {
                colors: {
                    primary: '#0149FF',
                    secondary: '#537CFF',
                    background: '#0d1116',
                    surface: '#0d1116',
                    warning: '#FF8A00',
                    error: '#FF0149',
                    pink: '#FF458B',
                    purple: '#502EFF',
                    yellow: '#FFC600',
                    orange: '#FFA800',
                    green: '#00B150',
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
            class: 'text-capitalize font-weight-bold',
            style: 'letter-spacing:0;',
        },
        VTooltip: {
            transition: 'fade-transition',
        },
        VChip: {
            rounded: 'lg',
        },
        VSelect: {
            rounded: 'lg',
        },
        VTextField: {
            rounded: 'lg',
            variant: 'outlined',
        },
    },
});
