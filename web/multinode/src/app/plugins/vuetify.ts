// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

import Vue from 'vue';
import Vuetify from 'vuetify/lib/framework';
import '@mdi/font/css/materialdesignicons.css';
import '@/styles/styles.scss';

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
                free:'#d6d6d6',
                trash: '#8fa7c6',
                overused: '#eb5757',
                success: '#00AC26',
                error: '#FF0149',
                warning: '#FFA800',
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
                free: '#d4effa',
                trash: '#9dc6fc',
                overused: '#ff4747',
                success: '#00AC26',
                error: '#FF0149',
                warning: '#FFA800',
                border: '#242d40',
                active: '#172135',
            },
        },
    },
});
