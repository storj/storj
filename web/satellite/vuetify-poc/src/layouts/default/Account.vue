// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-app>
        <default-bar />
        <account-nav />
        <default-view />
    </v-app>
</template>

<script setup lang="ts">
import { VApp } from 'vuetify/components';

import DefaultBar from './AppBar.vue';
import AccountNav from './AccountNav.vue';
import DefaultView from './View.vue';
</script>
