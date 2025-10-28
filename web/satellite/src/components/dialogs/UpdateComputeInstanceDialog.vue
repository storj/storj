// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-dialog
        v-model="model"
        scrollable
        max-width="400px"
        transition="fade-transition"
        :persistent="isLoading"
    >
        <v-card rounded="xlg" :loading="isLoading">
            <v-sheet>
                <v-card-item class="pa-6">
                    <template #prepend>
                        <v-sheet
                            class="border-sm d-flex justify-center align-center"
                            width="40"
                            height="40"
                            rounded="lg"
                        >
                            <component :is="Computer" :size="18" />
                        </v-sheet>
                    </template>

                    <v-card-title class="font-weight-bold">
                        Update Instance
                    </v-card-title>

                    <template #append>
                        <v-btn
                            :icon="X"
                            variant="text"
                            size="small"
                            color="default"
                            @click="model = false"
                        />
                    </template>
                </v-card-item>
            </v-sheet>

            <v-divider />

            <v-card-item class="px-6">
                <v-form ref="form" v-model="formValid" class="pt-2" @submit.prevent="updateInstance">
                    <v-select
                        v-model="instanceType"
                        class="mb-2"
                        label="New Instance Type"
                        placeholder="Choose new instance type"
                        :rules="[RequiredRule]"
                        :items="instanceTypes"
                        required
                    />
                </v-form>
            </v-card-item>
            <v-divider />
            <v-card-actions class="pa-6">
                <v-row>
                    <v-col>
                        <v-btn
                            variant="outlined"
                            color="default"
                            block
                            :disabled="isLoading"
                            @click="model = false"
                        >
                            Cancel
                        </v-btn>
                    </v-col>
                    <v-col>
                        <v-btn
                            color="primary"
                            variant="flat"
                            block
                            :disabled="!formValid"
                            :loading="isLoading"
                            @click="updateInstance"
                        >
                            Update
                        </v-btn>
                    </v-col>
                </v-row>
            </v-card-actions>
        </v-card>
    </v-dialog>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue';
import {
    VBtn,
    VCard,
    VCardActions,
    VCardItem,
    VCardTitle,
    VCol,
    VDialog,
    VDivider,
    VForm,
    VRow,
    VSelect,
    VSheet,
} from 'vuetify/components';
import { Computer, X } from 'lucide-vue-next';

import { useLoading } from '@/composables/useLoading';
import { useNotify } from '@/composables/useNotify';
import { RequiredRule } from '@/types/common';
import { useComputeStore } from '@/store/modules/computeStore';
import { AnalyticsErrorEventSource } from '@/utils/constants/analyticsEventNames';
import { Instance } from '@/types/compute';

const computeStore = useComputeStore();

const { isLoading, withLoading } = useLoading();
const notify = useNotify();

const props = defineProps<{
    instance: Instance
}>();

const model = defineModel<boolean>({ required: true });

const instanceType = ref<string>();
const formValid = ref(false);
const form = ref<VForm>();

const instanceTypes = computed<string[]>(() => computeStore.state.availableInstanceTypes);

function updateInstance(): void {
    if (!formValid.value) return;

    withLoading(async () => {
        if (!instanceType.value) return;

        try {
            await computeStore.updateInstanceType(props.instance.id, instanceType.value);

            notify.success('Instance updated successfully');
            model.value = false;
        } catch (error) {
            notify.notifyError(error, AnalyticsErrorEventSource.UPDATE_COMPUTE_INSTANCE_DIALOG);
        }
    });
}

watch(model, val => {
    if (!val) {
        form.value?.reset();
        instanceType.value = undefined;
    } else {
        withLoading(async () => {
            try {
                await computeStore.getAvailableInstanceTypes();
            } catch (error) {
                notify.notifyError(error, AnalyticsErrorEventSource.UPDATE_COMPUTE_INSTANCE_DIALOG);
            }
        });
    }
});
</script>
