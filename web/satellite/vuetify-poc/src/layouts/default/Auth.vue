// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-app>
        <auth-bar />
        <default-view class="signup-background" />
    </v-app>
</template>

<script setup lang="ts">
import { VApp } from 'vuetify/components';
import { onBeforeMount } from 'vue';
import { useRoute, useRouter } from 'vue-router';

import AuthBar from './AuthBar.vue';
import DefaultView from './View.vue';

import { useUsersStore } from '@/store/modules/usersStore';

const usersStore = useUsersStore();

const route = useRoute();
const router = useRouter();

onBeforeMount(() => {
    if (usersStore.state.user.id) {
        // user is already logged in
        router.replace('/projects');
        return;
    }
});
</script>
