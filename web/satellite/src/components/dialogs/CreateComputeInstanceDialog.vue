// Copyright (C) 2024 Storj Labs, Inc.
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
                        Create Instance
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
                <v-form ref="form" v-model="formValid" class="pt-2" @submit.prevent="addInstance">
                    <v-text-field
                        v-model="name"
                        class="mb-2"
                        variant="outlined"
                        :rules="[RequiredRule]"
                        label="Name"
                        placeholder="Enter your instance name"
                        :hide-details="false"
                        :maxlength="100"
                        required
                    />

                    <v-select
                        v-model="location"
                        class="mb-2"
                        label="Location"
                        placeholder="Choose location"
                        :rules="[RequiredRule]"
                        :items="locations.map(loc => ({ title: `${loc.name} - ${loc.label}`, value: loc.name }))"
                        item-value="value"
                        required
                    />

                    <v-select
                        v-model="instanceType"
                        class="mb-2"
                        label="Instance Type"
                        placeholder="Choose instance type"
                        :rules="[RequiredRule]"
                        :items="instanceTypes"
                        required
                    />

                    <v-select
                        v-model="image"
                        class="mb-2"
                        label="Image"
                        placeholder="Choose image"
                        :rules="[RequiredRule]"
                        :items="images"
                        required
                    />

                    <v-expansion-panels>
                        <v-expansion-panel eager static class="border border-opacity-25 rounded-lg" @group:selected="({value}) => onToggleOptional(value)">
                            <v-expansion-panel-title>
                                Optional params
                            </v-expansion-panel-title>
                            <v-expansion-panel-text>
                                <v-select
                                    v-model="sshKeys"
                                    label="SSH Keys"
                                    placeholder="Select SSH keys"
                                    :items="existingKeys.map(key => ({ title: key.name, value: key.id }))"
                                    item-title="title"
                                    item-value="value"
                                    multiple
                                    chips
                                    clearable
                                />

                                <v-text-field
                                    v-model="hostname"
                                    class="mb-2"
                                    variant="outlined"
                                    :rules="[HostnameRule]"
                                    label="Hostname"
                                    placeholder="Enter a hostname"
                                    :hide-details="false"
                                />

                                <v-text-field
                                    v-model="bootDiskSize"
                                    class="mb-2"
                                    variant="outlined"
                                    label="Boot Disk Size (GB)"
                                    placeholder="Enter a boot disk size"
                                    hide-details
                                    type="number"
                                    min="0"
                                />
                            </v-expansion-panel-text>
                        </v-expansion-panel>
                    </v-expansion-panels>
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
                            @click="addInstance"
                        >
                            Create
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
    VExpansionPanel,
    VExpansionPanelTitle,
    VExpansionPanels,
    VExpansionPanelText,
    VForm,
    VRow,
    VSelect,
    VSheet,
    VTextField,
} from 'vuetify/components';
import { Computer, X } from 'lucide-vue-next';

import { useLoading } from '@/composables/useLoading';
import { useNotify } from '@/composables/useNotify';
import { HostnameRule, RequiredRule } from '@/types/common';
import { useComputeStore } from '@/store/modules/computeStore';
import { SSHKey, Location } from '@/types/compute';
import { AnalyticsErrorEventSource } from '@/utils/constants/analyticsEventNames';

const computeStore = useComputeStore();

const { isLoading, withLoading } = useLoading();
const notify = useNotify();

const model = defineModel<boolean>({ required: true });

const name = ref<string>('');
const hostname = ref<string>('');
const instanceType = ref<string>();
const location = ref<string>();
const image = ref<string>();
const bootDiskSize = ref<string>();
const sshKeys = ref<string[]>([]);
const formValid = ref(false);
const form = ref<VForm>();

const locations = computed<Location[]>(() => computeStore.state.availableLocations);
const instanceTypes = computed<string[]>(() => computeStore.state.availableInstanceTypes);
const images = computed<string[]>(() => computeStore.state.availableImages);

const existingKeys = computed<SSHKey[]>(() => computeStore.state.sshKeys);

function addInstance(): void {
    if (!formValid.value) return;

    withLoading(async () => {
        if (!(instanceType.value && location.value && image.value)) return;

        try {
            await computeStore.createInstance({
                name: name.value,
                instanceType: instanceType.value,
                location: location.value,
                image: image.value,
                hostname: hostname.value ? hostname.value : undefined,
                bootDiskSizeGB: bootDiskSize.value ? parseInt(bootDiskSize.value) : undefined,
                sshKeys: sshKeys.value.length ? sshKeys.value : undefined,
            });

            notify.success('Instance created successfully');
            model.value = false;
        } catch (error) {
            notify.notifyError(error, AnalyticsErrorEventSource.CREATE_COMPUTE_INSTANCE_DIALOG);
        }
    });
}

function onToggleOptional(val: boolean): void {
    if (val) return;

    hostname.value = '';
    bootDiskSize.value = undefined;
    sshKeys.value = [];
}

watch(model, val => {
    if (!val) {
        form.value?.reset();
        name.value = '';
        hostname.value = '';
        instanceType.value = undefined;
        location.value = undefined;
        image.value = undefined;
        bootDiskSize.value = undefined;
        sshKeys.value = [];
    } else {
        withLoading(async () => {
            try {
                await Promise.all([
                    computeStore.getSSHKeys(),
                    computeStore.getAvailableImages(),
                    computeStore.getAvailableInstanceTypes(),
                    computeStore.getAvailableLocations(),
                ]);
            } catch (error) {
                notify.notifyError(error, AnalyticsErrorEventSource.CREATE_COMPUTE_INSTANCE_DIALOG);
            }
        });
    }
});
</script>

<style scoped lang="scss">
:deep(.v-expansion-panel-text__wrapper) {
    padding-left: 12px !important;
    padding-right: 12px !important;
    padding-bottom: 6px !important;
}
</style>
