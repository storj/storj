// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-app>
        <branded-loader v-if="isLoading" />
        <session-wrapper v-else>
            <app-bar show-nav-drawer-button />
            <slot name="nav" />
            <app-view />

            <UpgradeAccountDialog v-model="appStore.state.isUpgradeFlowDialogShown" :is-member-upgrade="isMemberAccount" />
            <PricingOptInDialog v-model="appStore.state.isPricingOptInDialogShown" />
            <browser-snackbar-component />
        </session-wrapper>
    </v-app>
</template>

<script setup lang="ts">
import { computed } from 'vue';
import { VApp } from 'vuetify/components';

import AppBar from './AppBar.vue';
import AppView from './View.vue';

import { useAppStore } from '@/store/modules/appStore';
import { useUsersStore } from '@/store/modules/usersStore';

import SessionWrapper from '@/components/utils/SessionWrapper.vue';
import BrandedLoader from '@/components/utils/BrandedLoader.vue';
import UpgradeAccountDialog from '@/components/dialogs/upgradeAccountFlow/UpgradeAccountDialog.vue';
import PricingOptInDialog from '@/components/dialogs/PricingOptInDialog.vue';
import BrowserSnackbarComponent from '@/components/BrowserSnackbarComponent.vue';

withDefaults(defineProps<{
    isLoading?: boolean,
}>(), {
    isLoading: false,
});

const appStore = useAppStore();
const usersStore = useUsersStore();

const isMemberAccount = computed<boolean>(() => usersStore.state.user.isMember);
</script>
