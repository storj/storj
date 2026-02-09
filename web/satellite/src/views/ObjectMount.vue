// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-container>
        <PageTitleComponent title="Object Mount" />
        <PageSubtitleComponent
            subtitle="Connect Storj with Object Mount by creating S3 credentials."
            :link="ObjectMountApp.docs"
        />

        <v-row class="mt-4 mb-6">
            <v-col cols="12" md="6" lg="6" xl="6">
                <v-card class="px-1 py-3 h-100 align-content-space-between flex-grow-1 d-flex flex-column">
                    <v-card-item class="pb-0">
                        <img
                            :src="ObjectMountApp.src"
                            alt="Storj"
                            width="48"
                            height="48"
                            class="rounded-md border pa-2 bg-background mr-3"
                        >
                    </v-card-item>

                    <v-card-item>
                        <h3 class="mb-2">Connect with Object Mount</h3>
                        <p class="text-high-emphasis">{{ ObjectMountApp.description }}</p>
                    </v-card-item>

                    <v-card-item class="bottom mt-auto">
                        <v-btn color="primary" :append-icon="ArrowRight" @click="onSetup">
                            Create S3 Credentials
                        </v-btn>
                        <v-btn
                            variant="outlined"
                            color="default"
                            class="ml-2"
                            :href="ObjectMountApp.docs"
                            :append-icon="SquareArrowOutUpRight"
                            target="_blank"
                            rel="noopener noreferrer"
                        >
                            Docs
                        </v-btn>
                    </v-card-item>
                </v-card>
            </v-col>
            <v-col cols="12" md="6" lg="6" xl="6">
                <v-card class="px-1 py-3 h-100 align-content-space-between flex-grow-1 d-flex flex-column">
                    <v-card-item class="pb-0">
                        <img
                            :src="OMIcon"
                            alt="OM"
                            width="48"
                            height="48"
                            class="rounded-md border pa-2 bg-background mr-3"
                        >
                    </v-card-item>

                    <v-card-item>
                        <h3 class="mb-2">Get Object Mount</h3>
                        <p class="text-high-emphasis">
                            Turn cloud storage into local storage. Work with remote files instantly – no downloading, syncing, or waiting.
                        </p>
                    </v-card-item>

                    <v-card-item class="bottom mt-auto">
                        <v-btn
                            variant="outlined"
                            color="default"
                            href="https://www.storj.io/pricing#object-mount"
                            :append-icon="SquareArrowOutUpRight"
                            target="_blank"
                            rel="noopener noreferrer"
                        >
                            View Pricing
                        </v-btn>
                        <v-btn
                            variant="outlined"
                            color="default"
                            class="ml-2"
                            :href="getInTouchURL"
                            :append-icon="SquareArrowOutUpRight"
                            target="_blank"
                            rel="noopener noreferrer"
                        >
                            Schedule Meeting
                        </v-btn>
                    </v-card-item>
                </v-card>
            </v-col>
        </v-row>

        <PageTitleComponent title="Credentials" />
        <AccessTableComponent :constant-search="ObjectMountApp.name" />
    </v-container>

    <AccessSetupDialog
        v-model="dialog"
        :app="ObjectMountApp"
        :access-name="ObjectMountApp.name"
        :docs-link="ObjectMountApp.docs"
        :default-step="SetupStep.ChooseFlowStep"
        :default-access-type="AccessType.S3"
    />
</template>

<script setup lang="ts">
import { computed, onBeforeMount, ref } from 'vue';
import { useRouter } from 'vue-router';
import {
    VCard,
    VCardItem,
    VCol,
    VContainer,
    VRow,
    VBtn,
} from 'vuetify/components';
import { ArrowRight, SquareArrowOutUpRight } from 'lucide-vue-next';

import { useConfigStore } from '@/store/modules/configStore';
import { ROUTES } from '@/router';
import { AccessType, SetupStep } from '@/types/setupAccess';
import { ObjectMountApp } from '@/types/applications';
import { usePreCheck } from '@/composables/usePreCheck';

import PageSubtitleComponent from '@/components/PageSubtitleComponent.vue';
import PageTitleComponent from '@/components/PageTitleComponent.vue';
import AccessSetupDialog from '@/components/dialogs/AccessSetupDialog.vue';
import AccessTableComponent from '@/components/AccessTableComponent.vue';

import OMIcon from '@/assets/apps/objectmount.svg';

const router = useRouter();
const { withTrialCheck, withManagedPassphraseCheck } = usePreCheck();

const configStore = useConfigStore();

const dialog = ref<boolean>(false);

const getInTouchURL = computed<string>(() => configStore.state.config.scheduleMeetingURL);

function onSetup(): void {
    withTrialCheck(() => { withManagedPassphraseCheck(() => {
        dialog.value = true;
    });});
}

onBeforeMount(() => {
    if (!configStore.isDefaultBrand) router.push({ name: ROUTES.Dashboard.name });
});
</script>
