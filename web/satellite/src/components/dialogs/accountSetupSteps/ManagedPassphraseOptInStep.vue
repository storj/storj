// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-container>
        <v-row justify="center">
            <v-col class="text-center py-4">
                <img height="50" width="50" src="@/assets/icon-change-password.svg" alt="Change password">
                <p class="text-overline mt-2 mb-1">
                    Project Encryption
                </p>
                <h2>Choose how to manage the encryption for your data.</h2>
            </v-col>
        </v-row>

        <v-row justify="center">
            <v-col cols="12" sm="10" md="6" lg="4">
                <v-card variant="outlined" rounded="xlg" class="h-100">
                    <div class="d-flex flex-column justify-space-between pa-6">
                        <h3 class="font-weight-black">Automatic <v-chip size="x-small" variant="outlined" color="secondary" class="ml-1 mb-1 font-weight-bold">Standard</v-chip></h3>
                        <p>{{ configStore.brandName }} securely manages the encryption and decryption of your project automatically.</p>
                        <p>
                            <v-chip rounded="md" class="text-caption mt-2 mb-3 font-weight-medium" color="secondary" variant="tonal" size="small">
                                Recommended for ease of use and teams
                            </v-chip>
                        </p>

                        <p class="text-body-2 my-2">
                            <b>Simple user experience</b><br>
                            Fewer steps to upload, download, manage, and browse your data.
                            No need to remember an additional encryption passphrase.
                        </p>

                        <p class="text-body-2 my-2">
                            <b>Easy team management</b><br>
                            Team members you invite will automatically have access to your project's data.
                        </p>

                        <p class="text-body-2 my-2">
                            <b>Full S3-compatibility</b><br>
                            Recommended for full S3-compatibility.
                        </p>

                        <p class="text-body-2 my-2">
                            <a class="link" href="https://storj.dev/dcs/api/s3/s3-compatibility/storj-vs-self-managed-encryption-s3-compatibility-differences" target="_blank" rel="noopener noreferrer">S3 compatibility differences</a>
                            •
                            <a class="link" @click="goToDocs">Encryption options documentation</a>
                        </p>

                        <v-btn
                            :disabled="selectedMode !== 'auto' && loading"
                            :loading="selectedMode === 'auto' && loading"
                            class="mt-4"
                            @click="onModeChosen('auto')"
                        >
                            <template #append>
                                <v-icon :icon="ArrowRight" />
                            </template>
                            Automatic
                        </v-btn>
                    </div>
                </v-card>
            </v-col>

            <v-col cols="12" sm="10" md="6" lg="4">
                <v-card variant="outlined" rounded="xlg" class="h-100">
                    <div class="d-flex flex-column justify-space-between pa-6">
                        <h3 class="font-weight-black primary">Self-managed <v-chip size="x-small" variant="outlined" class="ml-1 mb-1 font-weight-bold">Advanced</v-chip></h3>
                        <p>You are responsible for securely managing your own data encryption passphrase.</p>
                        <p>
                            <v-chip rounded="md" class="text-caption mt-2 mb-3 font-weight-medium" color="secondary" size="small" variant="tonal">
                                Best for control over your data encryption
                            </v-chip>
                        </p>

                        <p class="text-body-2 my-2">
                            <b>Passphrase experience</b><br>
                            You need to enter your passphrase each time you access your data.
                            If you forget the passphrase, you can't recover your data.
                        </p>

                        <p class="text-body-2 my-2">
                            <b>Manual team management</b><br>
                            Team members must share and enter the same encryption passphrase to access the data.
                        </p>

                        <p class="text-body-2 my-2">
                            <b>Limited S3-compatibility</b><br>
                            Increased control, limited S3-compatibility.
                        </p>

                        <p class="text-body-2 my-2">
                            <a class="link" href="https://storj.dev/dcs/api/s3/s3-compatibility/storj-vs-self-managed-encryption-s3-compatibility-differences" target="_blank" rel="noopener noreferrer">S3 compatibility differences</a>
                            •
                            <a class="link" @click="goToDocs">Encryption options documentation</a>
                        </p>

                        <v-btn
                            :disabled="selectedMode !== 'manual' && loading"
                            :loading="selectedMode === 'manual' && loading"
                            variant="outlined"
                            color="secondary"
                            class="mt-4"
                            @click="onModeChosen('manual')"
                        >
                            Self-managed
                            <template #append>
                                <v-icon :icon="ArrowRight" />
                            </template>
                        </v-btn>
                    </div>
                </v-card>
            </v-col>
        </v-row>
    </v-container>
</template>

<script setup lang="ts">
import { VBtn, VCard, VChip, VCol, VContainer, VIcon, VRow } from 'vuetify/components';
import { ref } from 'vue';
import { ArrowRight } from 'lucide-vue-next';

import { useProjectsStore } from '@/store/modules/projectsStore';
import { useUsersStore } from '@/store/modules/usersStore';
import { ManagePassphraseMode } from '@/types/projects';
import {
    AnalyticsEvent,
    PageVisitSource,
    SATELLITE_MANAGED_ENCRYPTION_DOCS_PAGE,
} from '@/utils/constants/analyticsEventNames';
import { useAnalyticsStore } from '@/store/modules/analyticsStore';
import { useConfigStore } from '@/store/modules/configStore';

const analyticsStore = useAnalyticsStore();
const projectsStore = useProjectsStore();
const userStore = useUsersStore();
const configStore = useConfigStore();

defineProps<{
    loading: boolean,
}>();

const emit = defineEmits<{
    next: [];
}>();

const selectedMode = ref<ManagePassphraseMode>();

function onModeChosen(mode: ManagePassphraseMode) {
    selectedMode.value = mode;
    emit('next');
}

async function setup() {
    await projectsStore.createDefaultProject(userStore.state.user.id, selectedMode.value === 'auto');
}

function goToDocs() {
    analyticsStore.pageVisit(SATELLITE_MANAGED_ENCRYPTION_DOCS_PAGE, PageVisitSource.DOCS);
    analyticsStore.eventTriggered(AnalyticsEvent.VIEW_DOCS_CLICKED);
    window.open(SATELLITE_MANAGED_ENCRYPTION_DOCS_PAGE, '_blank', 'noreferrer');
}

defineExpose({
    validate: () => !!selectedMode.value,
    setup,
});
</script>
