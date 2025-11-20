// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-dialog
        v-model="dialog"
        activator="parent"
        width="auto"
        max-width="450px"
        transition="fade-transition"
    >
        <v-card rounded="xlg">
            <v-sheet>
                <v-card-item class="pa-6">
                    <template #prepend>
                        <v-sheet
                            class="border-sm d-flex justify-center align-center"
                            width="40"
                            height="40"
                            rounded="lg"
                        >
                            <component :is="LockKeyhole" :size="18" />
                        </v-sheet>
                    </template>
                    <v-card-title class="font-weight-bold">
                        Project Encryption
                    </v-card-title>
                    <template #append>
                        <v-btn
                            :icon="X"
                            variant="text"
                            size="small"
                            color="default"
                            @click="dialog = false"
                        />
                    </template>
                </v-card-item>
            </v-sheet>

            <v-divider />

            <div class="pa-6">
                <v-row>
                    <v-col>
                        <p class="mb-2">Encryption method:</p>
                        <v-chip-group v-model="encryption" filter variant="tonal" column selected-class="font-weight-bold" mandatory>
                            <v-chip color="primary" value="auto" class="cursor-default" :disabled="encryption === 'manual'">
                                Automatic
                            </v-chip>
                            <v-chip color="primary" value="manual" class="cursor-default" :disabled="encryption === 'auto'">Manual</v-chip>

                            <v-divider thickness="0" class="my-1" />

                            <v-alert v-if="encryption === 'auto'" variant="tonal" color="default">
                                <p>
                                    <v-chip rounded="md" class="text-caption font-weight-medium" color="secondary" variant="tonal" size="small">
                                        Recommended for ease of use and teams
                                    </v-chip>
                                </p>
                                <p class="text-body-2 my-2 font-weight-bold">
                                    {{ configStore.brandName }} securely manages the encryption and decryption of your project automatically.
                                </p>
                                <p class="text-body-2 my-2">
                                    Fewer steps to upload, download, manage, and browse your data. No need to remember an additional encryption passphrase.
                                </p>
                                <p class="text-body-2 my-2">
                                    The team members will automatically have access to your project's data.
                                </p>
                                <p class="text-body-2 mt-2">
                                    <a class="link" @click="goToDocs">Learn more in the documentation.</a>
                                </p>
                            </v-alert>

                            <v-alert v-if="encryption === 'manual'" variant="tonal" color="default">
                                <p>
                                    <v-chip rounded="md" class="text-caption font-weight-medium" color="secondary" variant="tonal" size="small">
                                        Best for control over your data encryption
                                    </v-chip>
                                </p>
                                <p class="text-body-2 my-2 font-weight-bold">
                                    You are responsible for securely managing your own data encryption passphrase.
                                </p>
                                <p class="text-body-2 my-2">
                                    You will need to enter your passphrase each time you access your data. If you forget the passphrase, you can't recover your data.
                                </p>
                                <p class="text-body-2 my-2">
                                    Team members must share and enter the same encryption passphrase to access the data.
                                </p>
                                <p class="text-body-2 mt-2">
                                    <a href="" class="link">Learn more in the documentation.</a>
                                </p>
                            </v-alert>
                        </v-chip-group>

                        <v-alert type="info" variant="tonal" class="mt-4">
                            <p class="text-body-2">Encryption method is set at project creation and can't be changed. To use a different method, create a new project.</p>
                        </v-alert>
                    </v-col>
                </v-row>
            </div>

            <v-divider />

            <v-card-actions class="pa-6">
                <v-row>
                    <v-col>
                        <v-btn
                            variant="outlined"
                            color="default"
                            block
                            @click="dialog = false"
                        >
                            Close
                        </v-btn>
                    </v-col>

                    <v-col>
                        <v-btn
                            variant="flat"
                            block
                            :prepend-icon="Plus"
                            @click="dialog = false; emit('newProject')"
                        >
                            New Project
                        </v-btn>
                    </v-col>
                </v-row>
            </v-card-actions>
        </v-card>
    </v-dialog>
</template>

<script setup lang="ts">
import {
    VDialog,
    VCol,
    VRow,
    VDivider,
    VCard,
    VCardActions,
    VCardItem,
    VAlert,
    VBtn,
    VChip,
    VChipGroup,
    VCardTitle,
    VSheet,
} from 'vuetify/components';
import { computed, ref } from 'vue';
import { Plus, LockKeyhole, X } from 'lucide-vue-next';

import {
    AnalyticsEvent,
    PageVisitSource,
    SATELLITE_MANAGED_ENCRYPTION_DOCS_PAGE,
} from '@/utils/constants/analyticsEventNames';
import { useAnalyticsStore } from '@/store/modules/analyticsStore';
import { useProjectsStore } from '@/store/modules/projectsStore';
import { useConfigStore } from '@/store/modules/configStore';

const analyticsStore = useAnalyticsStore();
const projectsStore = useProjectsStore();
const configStore = useConfigStore();

const emit = defineEmits(['newProject']);

const dialog = ref(false);

const encryption = computed(() => projectsStore.state.selectedProjectConfig.hasManagedPassphrase ? 'auto' : 'manual');

function goToDocs() {
    analyticsStore.pageVisit(SATELLITE_MANAGED_ENCRYPTION_DOCS_PAGE, PageVisitSource.DOCS);
    analyticsStore.eventTriggered(AnalyticsEvent.VIEW_DOCS_CLICKED);
    window.open(SATELLITE_MANAGED_ENCRYPTION_DOCS_PAGE, '_blank', 'noreferrer');
}
</script>
