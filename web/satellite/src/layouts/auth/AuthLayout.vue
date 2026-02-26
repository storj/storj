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
import { useRoute, useRouter } from 'vue-router';

import { useUsersStore } from '@/store/modules/usersStore';
import { useConfigStore } from '@/store/modules/configStore';
import { ROUTES } from '@/router';

import AppView from '@/layouts/shared/View.vue';
import AuthBar from '@/layouts/auth/AuthBar.vue';

const usersStore = useUsersStore();
const configStore = useConfigStore();

const route = useRoute();
const router = useRouter();

onBeforeMount(() => {
    if (usersStore.state.user.id) {
        // user is already logged in
        router.replace(ROUTES.Projects.path);
        return;
    }
    const loginURL = configStore.state.config.primaryAuthLoginURL;
    const ssoFailed = route.query['sso_failed'];
    if (loginURL && !ssoFailed && route.path !== ROUTES.AuthError.path) {
        window.location.href = loginURL;
    }
});
</script>
