// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-app>
        <auth-bar />
        <app-view />
    </v-app>
</template>

<script setup lang="ts">
import { VApp } from 'vuetify/components';
import { onBeforeMount } from 'vue';
import { useRouter } from 'vue-router';

import { useUsersStore } from '@/store/modules/usersStore';
import { ROUTES } from '@/router';

import AppView from '@/layouts/shared/View.vue';
import AuthBar from '@/layouts/auth/AuthBar.vue';

const usersStore = useUsersStore();

const router = useRouter();

onBeforeMount(() => {
    if (usersStore.state.user.id) {
        // user is already logged in
        router.replace(ROUTES.Projects.path);
    }
});
</script>
