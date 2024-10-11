// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

import Vue from 'vue';
import Vuetify from 'vuetify/lib/framework';
import '@mdi/font/css/materialdesignicons.css';

Vue.use(Vuetify);

export default new Vuetify({
    theme: {
        options: {
            customProperties: true,
        },
        themes: {
            light: {
                primary: '#0059d0',
                secondary: '#091C45',
                background: '#fcfcfd',
                text: '#586474',
                blue2: '#004199',
                header: '#131d3a',
                disabled: '#dadde5',
                active2: '#f6f7f8',
                background2: '#f0f6ff',
                // surface: '#FFF',
                // info: '#0059D0',
                // help: '#FFA800',
                // success: '#00AC26',
                // warning: '#FF7F00',
                // error: '#FF0149',
                // purple: '#7B61FF',
                // purple2: '#502EFF',
                // blue7: '#090920',
                // blue6:  '#091c45',
                // blue5: '#0218A7',
                // blue4: '#0059D0',
                // blue2: '#003ACD',
                // yellow: '#FFC600',
                // yellow2: '#FFB018',
                // orange: '#FFA800',
                // green: '#00E366',
                // paragraph: '#283968',
                border: '#e1e3e6',
                active: '#e7e9eb',
            },
            dark: {
                primary: '#0052FF',
                secondary: '#537CFF',
                background: '#000a20',
                text: '#ffffff',
                blue2: '#024deb',
                header: '#ffffff',
                disabled: '#252b38',
                active2: '#0a152a',
                background2: '#0a152a',
                // surface: '#000b21',
                // success: '#00AC26',
                // help: '#FFC600',
                // error: '#FF0149',
                // purple: '#A18EFF',
                // purple2: '#A18EFF',
                // blue7: '#090920',
                // blue6:  '#091c45',
                // blue5: '#2196f3',
                // blue4: '#0059D0',
                // blue2: '#003ACD',
                // yellow: '#FFC600',
                // yellow2: '#FFB018',
                // orange: '#FFA800',
                // warning: '#FF8A00',
                // green: '#00E366',
                border: '#242d40',
                active: '#172135',
            },
        },
    },
});
