// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-app>
        <auth-bar />
        <default-view class="signup-background"/>
    </v-app>
</template>

<script setup lang="ts">
import { VApp } from 'vuetify/components';

import AuthBar from './AuthBar.vue';
import DefaultView from './View.vue';
</script>
