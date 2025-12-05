// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-app>
        <session-wrapper>
            <default-bar show-nav-drawer-button />
            <account-nav />
            <default-view />

            <UpgradeAccountDialog v-model="appStore.state.isUpgradeFlowDialogShown" :is-member-upgrade="isMemberAccount" />
            <browser-snackbar-component />
        </session-wrapper>
    </v-app>
</template>

<script setup lang="ts">
import { VApp } from 'vuetify/components';
import { computed, onBeforeUnmount } from 'vue';

import DefaultBar from './AppBar.vue';
import AccountNav from './AccountNav.vue';
import DefaultView from './View.vue';

import { useAppStore } from '@/store/modules/appStore';
import { useUsersStore } from '@/store/modules/usersStore';

import SessionWrapper from '@/components/utils/SessionWrapper.vue';
import UpgradeAccountDialog from '@/components/dialogs/upgradeAccountFlow/UpgradeAccountDialog.vue';
import BrowserSnackbarComponent from '@/components/BrowserSnackbarComponent.vue';

const appStore = useAppStore();
const usersStore = useUsersStore();

const isMemberAccount = computed<boolean>(() => usersStore.state.user.isMember);

onBeforeUnmount(() => {
    appStore.toggleHasJustLoggedIn(false);
});
</script>
