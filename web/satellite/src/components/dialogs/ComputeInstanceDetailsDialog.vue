// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-dialog
        v-model="model"
        max-width="400px"
        min-width="320px"
        transition="fade-transition"
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
                        Instance Details
                    </v-card-title>

                    <template #append>
                        <v-btn
                            :icon="X"
                            variant="text"
                            size="small"
                            color="default"
                            :disabled="isLoading"
                            @click="model = false"
                        />
                    </template>
                </v-card-item>
            </v-sheet>

            <v-divider />

            <v-card-item>
                <v-row>
                    <v-col>
                        <v-list class="pa-0">
                            <v-list-item class="px-0 pb-4" :prepend-icon="FolderPen">
                                <v-list-item-title class="text-body-2 text-medium-emphasis">Name</v-list-item-title>
                                <v-list-item-subtitle class="mt-1">
                                    {{ displayed.name }}
                                </v-list-item-subtitle>
                            </v-list-item>

                            <v-list-item class="px-0 pb-4" :prepend-icon="ScreenShare">
                                <v-list-item-title class="text-body-2 text-medium-emphasis">Hostname</v-list-item-title>
                                <v-list-item-subtitle class="mt-1">
                                    {{ displayed.hostname }}
                                </v-list-item-subtitle>
                            </v-list-item>

                            <v-list-item class="px-0 pb-4" :prepend-icon="Loader">
                                <v-list-item-title class="text-body-2 text-medium-emphasis">Status</v-list-item-title>
                                <v-list-item-subtitle class="mt-1">
                                    {{ displayed.status }}
                                </v-list-item-subtitle>
                            </v-list-item>

                            <v-list-item class="px-0 pb-4" :prepend-icon="MapPinHouse">
                                <v-list-item-title class="text-body-2 text-medium-emphasis">IPv4 Address</v-list-item-title>
                                <v-list-item-subtitle class="mt-1">
                                    {{ displayed.ipv4Address }}
                                </v-list-item-subtitle>
                            </v-list-item>

                            <v-list-item class="px-0 pb-4" :prepend-icon="CalendarCheck">
                                <v-list-item-title class="text-body-2 text-medium-emphasis">Created At</v-list-item-title>
                                <v-list-item-subtitle class="mt-1">
                                    {{ Time.formattedDate(displayed.created) }}
                                </v-list-item-subtitle>
                            </v-list-item>
                        </v-list>
                    </v-col>
                </v-row>
            </v-card-item>

            <v-divider />

            <v-card-actions class="pa-6">
                <v-btn
                    variant="outlined"
                    color="default"
                    class="me-3"
                    min-width="100"
                    block
                    @click="model = false"
                >
                    Close
                </v-btn>
            </v-card-actions>
        </v-card>
    </v-dialog>
</template>

<script setup lang="ts">
import { computed, watch } from 'vue';
import {
    VDialog,
    VCard,
    VCardTitle,
    VCardActions,
    VCardItem,
    VBtn,
    VDivider,
    VSheet,
    VCol,
    VRow,
    VList,
    VListItem,
    VListItemTitle,
    VListItemSubtitle,
} from 'vuetify/components';
import { CalendarCheck, Computer, FolderPen, Loader, MapPinHouse, ScreenShare, X } from 'lucide-vue-next';

import { useLoading } from '@/composables/useLoading';
import { useComputeStore } from '@/store/modules/computeStore';
import { useNotify } from '@/composables/useNotify';
import { AnalyticsErrorEventSource } from '@/utils/constants/analyticsEventNames';
import { Instance } from '@/types/compute';
import { Time } from '@/utils/time';

const notify = useNotify();
const { isLoading, withLoading } = useLoading();

const computeStore = useComputeStore();

const props = defineProps<{
    instance: Instance;
}>();

const model = defineModel<boolean>({ required: true });

const displayed = computed<Instance>(() => computeStore.state.instances.find(i => i.id === props.instance.id) || props.instance);

watch(model, (newVal) => {
    if (!newVal) return;

    withLoading(async () => {
        try {
            await computeStore.getInstance(props.instance.id);
        } catch (error) {
            notify.error(error, AnalyticsErrorEventSource.COMPUTE_INSTANCE_DETAILS_MODAL);
        }
    });
});
</script>

<style scoped lang="scss">
:deep(.v-list-item__spacer) {
    width: 12px !important;
}
</style>
