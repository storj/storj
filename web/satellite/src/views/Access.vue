// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-container>
        <trial-expiration-banner v-if="isTrialExpirationBanner && isUserProjectOwner" :expired="isExpired" />

        <PageTitleComponent title="Access Keys" />
        <PageSubtitleComponent
            :subtitle="subtitle"
            :link="configStore.isDefaultBrand ? 'https://docs.storj.io/dcs/access' : undefined"
        />

        <v-col>
            <v-row class="mt-1 mb-2">
                <v-btn :prepend-icon="CirclePlus" @click="onCreateAccess">
                    New Access Key
                </v-btn>
            </v-row>
        </v-col>

        <AccessTableComponent />
    </v-container>
    <AccessSetupDialog
        v-model="dialog"
        docs-link="https://docs.storj.io/dcs/access"
        :default-step="SetupStep.ChooseAccessStep"
    />
</template>

<script setup lang="ts">
import { computed, ref } from 'vue';
import {
    VContainer,
    VCol,
    VRow,
    VBtn,
} from 'vuetify/components';
import { CirclePlus } from 'lucide-vue-next';

import { usePreCheck } from '@/composables/usePreCheck';
import { SetupStep } from '@/types/setupAccess';
import { useConfigStore } from '@/store/modules/configStore';

import AccessSetupDialog from '@/components/dialogs/AccessSetupDialog.vue';
import PageTitleComponent from '@/components/PageTitleComponent.vue';
import PageSubtitleComponent from '@/components/PageSubtitleComponent.vue';
import AccessTableComponent from '@/components/AccessTableComponent.vue';
import TrialExpirationBanner from '@/components/TrialExpirationBanner.vue';

const configStore = useConfigStore();

const dialog = ref<boolean>(false);

const { isTrialExpirationBanner, isUserProjectOwner, isExpired, withTrialCheck, withManagedPassphraseCheck } = usePreCheck();

const subtitle = computed<string>(() => {
    if (configStore.state.config.hideUplinkBehavior)
        return 'Create S3 Credentials.';

    return 'Create Access Grants, S3 Credentials, and API Keys.';
});

/**
 * Starts create access grant flow if user's free trial is not expired.
 */
function onCreateAccess(): void {
    withTrialCheck(() => { withManagedPassphraseCheck(() => {
        dialog.value = true;
    });});
}
</script>
