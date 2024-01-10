// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-dialog v-model="dialog" activator="parent" width="auto" transition="fade-transition">
        <v-card rounded="xlg">
            <v-sheet>
                <v-card-item class="pl-7 py-4">
                    <template #prepend>
                        <v-card-title class="font-weight-bold">
                            Project Limits
                        </v-card-title>
                    </template>

                    <template #append>
                        <v-btn icon="$close" variant="text" size="small" color="default" @click="dialog = false" />
                    </template>
                </v-card-item>
            </v-sheet>

            <v-divider />

            <v-form v-model="valid" class="pa-7">
                <v-row>
                    <v-col cols="12">
                        <p>Enter limits for this project.</p>
                    </v-col>
                </v-row>

                <v-row>
                    <v-col cols="12" sm="6">
                        <v-text-field
                            v-model="buckets" :rules="limitRules" label="Buckets" suffix="Buckets" variant="outlined" :disabled="isLoading"
                            hide-details="auto"
                        />
                    </v-col>
                    <!-- TODO: Implement unit selection (GB, TB, etc.) -->
                    <v-col cols="12" sm="6">
                        <v-text-field
                            v-model="storage" :rules="limitRules" label="Storage" suffix="Bytes" variant="outlined" :disabled="isLoading"
                            hide-details="auto"
                        />
                    </v-col>
                    <v-col cols="12" sm="6">
                        <v-text-field
                            v-model="egress" :rules="limitRules" label="Download per month" suffix="Bytes" variant="outlined" :disabled="isLoading"
                            hide-details="auto"
                        />
                    </v-col>
                    <v-col cols="12" sm="6">
                        <v-text-field
                            v-model="segments" :rules="limitRules" label="Segments" variant="outlined" :disabled="isLoading"
                            hide-details="auto"
                        />
                    </v-col>
                    <v-col cols="12" sm="6">
                        <v-text-field v-model="rate" :rules="limitRules" label="Rate" variant="outlined" :disabled="isLoading" hide-details="auto" />
                    </v-col>
                    <v-col cols="12" sm="6">
                        <v-text-field v-model="burst" :rules="limitRules" label="Burst" variant="outlined" :disabled="isLoading" hide-details="auto" />
                    </v-col>
                </v-row>

                <v-row>
                    <v-col cols="12">
                        <v-text-field
                            :model-value="appStore.state.selectedProject?.id" label="Project ID" variant="solo-filled" flat readonly
                            hide-details="auto"
                        />
                    </v-col>
                </v-row>
            </v-form>

            <v-divider />

            <v-card-actions class="pa-7">
                <v-row>
                    <v-col>
                        <v-btn variant="outlined" color="default" block @click="dialog = false">Cancel</v-btn>
                    </v-col>
                    <v-col>
                        <v-btn color="primary" variant="flat" block :loading="isLoading" @click="updateLimits">Save</v-btn>
                    </v-col>
                </v-row>
            </v-card-actions>
        </v-card>
    </v-dialog>
</template>

<script setup lang="ts">
import { onBeforeMount, ref, computed, watch } from 'vue';
import { useRouter } from 'vue-router';
import {
    VDialog,
    VCard,
    VSheet,
    VCardItem,
    VCardTitle,
    VBtn,
    VDivider,
    VForm,
    VRow,
    VCol,
    VTextField,
    VCardActions,
} from 'vuetify/components';

import { useAppStore } from '@/store/app';
import { useNotificationsStore } from '@/store/notifications';
import { RequiredRule, ValidationRule } from '@/types/common';

const valid = ref<boolean>(false);
const isLoading = ref<boolean>(false);

const appStore = useAppStore();
const notify = useNotificationsStore();
const router = useRouter();

const dialog = ref<boolean>(false);

const buckets = ref<number>(0);
const storage = ref<number>(0);
const egress = ref<number>(0);
const segments = ref<number>(0);
const rate = ref<number>(0);
const burst = ref<number>(0);

async function updateLimits() {
    if (!valid.value) {
        return;
    }
    isLoading.value = true;
    try {
        const updateReq = {
            maxBuckets: Number(buckets.value),
            storageLimit: Number(storage.value),
            bandwidthLimit: Number(egress.value),
            segmentLimit: Number(segments.value),
            rateLimit: Number(rate.value),
            burstLimit: Number(burst.value),
        };

        await appStore.updateProjectLimits(appStore.state.selectedProject?.id || '', updateReq);
        notify.notifySuccess('Successfully updated project limits.');
    } catch (error) {
        notify.notifyError(`Error updating project limits. ${error.message}`);
    }
    isLoading.value = false;
    dialog.value = false;
}

/**
 * Returns an array of validation rules applied to the text input.
 */
const limitRules = computed<ValidationRule<string>[]>(() => {
    return [
        RequiredRule,
        v => !(isNaN(+v) || isNaN(parseFloat(v))) || 'Invalid number',
        v => (parseFloat(v) >= 0) || 'Number must be zero or greater',
    ];
});

function setInputFields() {
    const p = appStore.state.selectedProject;
    if (!p) {
        return;
    }
    buckets.value = p.maxBuckets || 0;
    storage.value = p.storageLimit || 0;
    egress.value = p.bandwidthLimit || 0;
    segments.value = p.segmentLimit || 0;
    rate.value = p.rateLimit || 0;
    burst.value = p.burstLimit || 0;
}

watch(dialog, (shown) => {
    if (!shown || !appStore.state.selectedProject) {
        return;
    }
    setInputFields();
});

onBeforeMount(() => {
    if (!appStore.state.selectedProject) {
        router.push('/accounts');
        return;
    }
    setInputFields();
});
</script>