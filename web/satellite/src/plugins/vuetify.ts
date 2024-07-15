// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

import 'vuetify/styles';
import '@fontsource-variable/inter';
import { createVuetify } from 'vuetify';
import { md3 } from 'vuetify/blueprints';
import { aliases, mdi } from 'vuetify/iconsets/mdi-svg';

import '@/styles/styles.scss';
import { THEME_OPTIONS } from '@/utils/constants/theme';

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
        },
        VBtn: {
            density: 'default',
            rounded: 'lg',
            class: 'text-none font-weight-bold',
            style: 'letter-spacing:0;',
        },
        VTooltip: {
            transition: 'fade-transition',
            rounded: 'lg',
        },
        VSelect: {
            // rounded: 'lg',
            variant: 'outlined',
            color: 'secondary',
        },
        VTextField: {
            rounded: 'md',
            variant: 'outlined',
            color: 'secondary',
            centerAffix: true, // Vertically align append and prepend in the center.
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
            rounded: 'xlg',
        },
        VTable: {
            class: 'elevation-0',
        },
        VCheckbox: {
            color: 'primary',
            class: 'ml-n1',
        },
        VAlert: {
            rounded: 'xlg',
        },
        VChip: {
            rounded: 'md',
        },
        VRadio: {
            color: 'primary',
        },
    },
});
