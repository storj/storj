// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-container>
        <trial-expiration-banner v-if="isTrialExpirationBanner" :expired="isExpired" />

        <PageTitleComponent title="Access Keys" />
        <PageSubtitleComponent subtitle="Create Access Grants, S3 Credentials, and API Keys." link="https://docs.storj.io/dcs/access" />

        <v-col>
            <v-row class="mt-2 mb-4">
                <v-btn @click="onCreateAccess">
                    <IconNew class="mr-2" size="16" bold />
                    New Access Key
                </v-btn>
            </v-row>
        </v-col>

        <AccessTableComponent />
    </v-container>
    <NewAccessDialog v-model="dialog" />
</template>

<script setup lang="ts">
import { ref } from 'vue';
import {
    VContainer,
    VCol,
    VRow,
    VBtn,
} from 'vuetify/components';

import { useTrialCheck } from '@/composables/useTrialCheck';

import NewAccessDialog from '@/components/dialogs/CreateAccessDialog.vue';
import PageTitleComponent from '@/components/PageTitleComponent.vue';
import PageSubtitleComponent from '@/components/PageSubtitleComponent.vue';
import AccessTableComponent from '@/components/AccessTableComponent.vue';
import IconNew from '@/components/icons/IconNew.vue';
import TrialExpirationBanner from '@/components/TrialExpirationBanner.vue';

const dialog = ref<boolean>(false);

const { isTrialExpirationBanner, isExpired, withTrialCheck } = useTrialCheck();

/**
 * Starts create access grant flow if user's free trial is not expired.
 */
function onCreateAccess(): void {
    withTrialCheck(() => {
        dialog.value = true;
    });
}
</script>
