// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-app>
        <session-wrapper>
            <default-bar />
            <default-view />

            <UpgradeAccountDialog v-model="appStore.state.isUpgradeFlowDialogShown" />
            <browser-snackbar-component />
        </session-wrapper>
    </v-app>
</template>

<script setup lang="ts">
import { VApp } from 'vuetify/components';
import { onBeforeUnmount } from 'vue';
import { useRouter } from 'vue-router';

import DefaultBar from './AppBar.vue';
import DefaultView from './View.vue';

import { useAppStore } from '@poc/store/appStore';
import { useUsersStore } from '@/store/modules/usersStore';
import { useNotify } from '@/utils/hooks';
import { useProjectsStore } from '@/store/modules/projectsStore';
import { useAnalyticsStore } from '@/store/modules/analyticsStore';

import SessionWrapper from '@poc/components/utils/SessionWrapper.vue';
import UpgradeAccountDialog from '@poc/components/dialogs/upgradeAccountFlow/UpgradeAccountDialog.vue';
import BrowserSnackbarComponent from '@poc/components/BrowserSnackbarComponent.vue';

const appStore = useAppStore();
const usersStore = useUsersStore();
const projectsStore = useProjectsStore();
const analyticsStore = useAnalyticsStore();

const notify = useNotify();
const router = useRouter();

onBeforeUnmount(() => {
    appStore.toggleHasJustLoggedIn(false);
});
</script>
