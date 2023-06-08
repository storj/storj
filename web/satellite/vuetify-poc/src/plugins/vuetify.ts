// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

import { Blueprint, createVuetify } from 'vuetify';
import { md3 } from 'vuetify/lib/blueprints/index.mjs';
import { VDataTable } from 'vuetify/lib/labs/VDataTable/index.mjs';
import '@mdi/font/css/materialdesignicons.css'
import 'vuetify/styles';

export default createVuetify({
    blueprint: md3 as Blueprint,
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
                    surface: '#fff',
                    purple: '#7B61FF',
                    blue6:  '#091c45',
                    blue5: '#0218A7',
                    blue4: '#0059D0',
                },
            },
            dark: {
                colors: {
                    primary: '#0149FF',
                    secondary: '#537CFF',
                    background: '#0c121d',
                    surface: '#0c121d',
                    purple: '#7B61FF',
                    blue6:  '#091c45',
                    blue5: '#2196f3',
                    blue4: '#0059D0',
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
            elevation: 0,
            density: 'default',
            rounded: 'lg',
        },
    },
});
