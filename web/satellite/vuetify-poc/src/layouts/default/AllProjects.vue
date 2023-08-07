// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-app>
        <default-bar />
        <default-view />
    </v-app>
</template>

<script setup lang="ts">
import { VApp } from 'vuetify/components';
import { onBeforeMount } from 'vue';

import DefaultBar from './AppBar.vue';
import DefaultView from './View.vue';

import { useUsersStore } from '@/store/modules/usersStore';

const usersStore = useUsersStore();

/**
 * Lifecycle hook after initial render.
 * Pre-fetches user`s and project information.
 */
onBeforeMount(async () => {
    try {
        await usersStore.getSettings();
    } catch (error) { /* empty */ }
});
</script>
