// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-dialog
        v-model="model"
        max-width="420px"
        transition="fade-transition"
        :persistent="false"
    >
        <v-card :loading="isLoading">
            <v-card-item class="pa-6">
                <template #prepend>
                    <v-sheet
                        class="border-sm d-flex justify-center align-center"
                        variant="tonal"
                        width="40"
                        height="40"
                        rounded="lg"
                    >
                        <icon-distribution size="18" />
                    </v-sheet>
                </template>
                <v-card-title class="font-weight-bold">
                    Geographic Distribution
                </v-card-title>
                <template #append>
                    <v-btn
                        id="close-geo-distribution"
                        :icon="X"
                        variant="text"
                        size="small"
                        color="default"
                        @click="model = false"
                    />
                </template>
            </v-card-item>

            <v-divider />

            <div class="pa-6">
                <template v-if="mapURL">
                    <img class="w-100" :src="mapURL" alt="map">
                    <p class="font-weight-bold my-4">
                        This map shows the real-time locations of this object’s pieces.
                    </p>
                </template>
                <p>
                    {{ configStore.brandName }} splits objects into smaller pieces, then distributes those pieces
                    over a global network of nodes and recompiles them securely on download.
                </p>
            </div>

            <v-divider />

            <v-card-actions class="pa-6">
                <v-row>
                    <v-col>
                        <v-btn
                            variant="outlined"
                            color="default"
                            block
                            @click="model = false"
                        >
                            Close
                        </v-btn>
                    </v-col>
                    <v-col>
                        <v-btn
                            variant="outlined"
                            block
                            color="default"
                            link
                            href="https://docs.storj.io/learn#what-happens-when-you-upload"
                            target="_blank"
                            rel="noopener noreferrer"
                            :append-icon="SquareArrowOutUpRight"
                        >
                            Learn more
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
    VDialog,
    VCard,
    VCardItem,
    VSheet,
    VCardTitle,
    VDivider,
    VCardActions,
    VRow,
    VCol,
    VBtn,
} from 'vuetify/components';
import { SquareArrowOutUpRight, X } from 'lucide-vue-next';

import { useBucketsStore } from '@/store/modules/bucketsStore';
import { useObjectBrowserStore } from '@/store/modules/objectBrowserStore';
import { useLoading } from '@/composables/useLoading';
import { useNotify } from '@/composables/useNotify';
import { useLinksharing } from '@/composables/useLinksharing';
import { AnalyticsErrorEventSource } from '@/utils/constants/analyticsEventNames';
import { useConfigStore } from '@/store/modules/configStore';

import IconDistribution from '@/components/icons/IconDistribution.vue';

const model = defineModel<boolean>({ required: true });

const obStore = useObjectBrowserStore();
const bucketsStore = useBucketsStore();
const configStore = useConfigStore();

const notify = useNotify();
const { isLoading, withLoading } = useLoading();
const { getObjectDistributionMap } = useLinksharing();

const mapURL = ref<string>('');

/**
 * Returns bucket name from store.
 */
const bucket = computed<string>(() => {
    return bucketsStore.state.fileComponentBucketName;
});

/**
 * Retrieve the encoded objectpath.
 */
const encodedFilePath = computed<string>(() => {
    return encodeURIComponent(`${bucket.value}/${obStore.state.objectPathForModal.trim()}`);
});

/**
 * Downloads object geographic distribution map.
 */
async function getMap(): Promise<void> {
    await withLoading(async () => {
        try {
            const blob = await getObjectDistributionMap(encodedFilePath.value);
            mapURL.value = URL.createObjectURL(blob);
        } catch (error) {
            notify.notifyError(error, AnalyticsErrorEventSource.GALLERY_VIEW);
        }
    });
}

watch(model, value => {
    if (value) getMap();
    else mapURL.value = '';
});
</script>
