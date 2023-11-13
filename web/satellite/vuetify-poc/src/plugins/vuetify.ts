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

import '@poc/styles/styles.scss';
import { THEME_OPTIONS } from '@poc/plugins/theme';

// https://vuetifyjs.com/en/introduction/why-vuetify/#feature-guides
export default createVuetify({
    // Use blueprint for Material Design 3
    blueprint: md3,
    theme: THEME_OPTIONS,
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
