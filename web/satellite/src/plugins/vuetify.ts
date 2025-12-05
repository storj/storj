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
