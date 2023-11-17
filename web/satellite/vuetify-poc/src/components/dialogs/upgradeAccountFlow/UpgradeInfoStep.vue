// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-row>
        <v-col v-if="!smAndDown" cols="6">
            <h4 class="font-weight-bold my-2">Free Account</h4>
            <v-btn
                block
                disabled
                color="default"
            >
                Current
            </v-btn>
            <v-card class="my-4">
                <InfoBullet title="Projects" :info="freeProjects" />
                <InfoBullet title="Storage" :info="`${freeUsageValue(user.projectStorageLimit)} limit`" />
                <InfoBullet title="Download" :info="`${freeUsageValue(user.projectBandwidthLimit)} limit`" />
                <InfoBullet title="Segments" :info="`${user.projectSegmentLimit.toLocaleString()} segments limit`" />
                <InfoBullet title="Link Sharing" info="Link sharing with Storj domain" />
            </v-card>
        </v-col>
        <v-col :cols="smAndDown ? 12 : '6'">
            <h4 class="font-weight-bold my-2">Pro Account</h4>
            <v-btn
                class="mb-1"
                block
                :loading="loading"
                append-icon="mdi-arrow-right"
                @click="emit('upgrade')"
            >
                Upgrade
            </v-btn>
            <v-card class="my-4">
                <InfoBullet is-pro title="Projects" :info="projectsInfo" />
                <InfoBullet is-pro :title="storagePrice" :info="storagePriceInfo" />
                <InfoBullet is-pro :title="downloadPrice" :info="downloadInfo">
                    <template v-if="downloadMoreInfo" #moreInfo>
                        <p>{{ downloadMoreInfo }}</p>
                    </template>
                </InfoBullet>
                <InfoBullet is-pro title="Segments" :info="segmentInfo">
                    <template #moreInfo>
                        <a
                            class="link"
                            href="https://docs.storj.io/dcs/billing-payment-and-accounts-1/pricing/billing-and-payment"
                            target="_blank"
                            rel="noopener noreferrer"
                        >
                            Learn more about segments
                        </a>
                    </template>
                </InfoBullet>
                <InfoBullet is-pro title="Secure Custom Domains (HTTPS)" info="Link sharing with your domain" />
                <InfoBullet is-pro title="Team" info="Share projects and collaborate" />
            </v-card>
        </v-col>
    </v-row>
</template>

<script setup lang="ts">
import { computed, onBeforeMount, ref } from 'vue';
import { VBtn, VCol, VRow, VCard } from 'vuetify/components';
import { useDisplay } from 'vuetify';

import { useUsersStore } from '@/store/modules/usersStore';
import { useNotify } from '@/utils/hooks';
import { User } from '@/types/users';
import { Size } from '@/utils/bytesSize';

import InfoBullet from '@poc/components/dialogs/upgradeAccountFlow/InfoBullet.vue';

const usersStore = useUsersStore();
const notify = useNotify();
const { smAndDown } = useDisplay();

const props = defineProps<{
    loading: boolean;
}>();

const emit = defineEmits<{
    upgrade: [];
}>();

const storagePrice = ref<string>('Storage $0.004 GB / month');
const storagePriceInfo = ref<string>('25 GB free included');
const segmentInfo = ref<string>('$0.0000088 segment per month');
const projectsInfo = ref<string>('3 projects + more on request');
const downloadPrice = ref<string>('Download $0.007 GB');
const downloadInfo = ref<string>('25 GB free every month');
const downloadMoreInfo = ref<string>('');

/**
 * Returns user entity from store.
 */
const user = computed((): User => {
    return usersStore.state.user;
});

/**
 * Returns formatted free projects count.
 */
const freeProjects = computed((): string => {
    return `${user.value.projectLimit} project${user.value.projectLimit > 1 ? 's' : ''}`;
});

/**
 * Returns formatted free usage value.
 */
function freeUsageValue(value: number): string {
    const size = new Size(value);
    return `${size.formattedBytes} ${size.label}`;
}

/**
 * Lifecycle hook before initial render.
 * If applicable, loads additional clarifying text based on user partner.
 */
onBeforeMount(async () => {
    try {
        const partner = usersStore.state.user.partner;
        const config = (await import('@poc/components/dialogs/upgradeAccountFlow/upgradeConfig.json')).default;
        if (partner && config[partner]) {
            if (config[partner].storagePrice) {
                storagePrice.value = config[partner].storagePrice;
            }

            if (config[partner].downloadInfo) {
                downloadInfo.value = config[partner].downloadInfo;
            }

            if (config[partner].downloadMoreInfo) {
                downloadMoreInfo.value = config[partner].downloadMoreInfo;
            }
        }
    } catch (e) {
        notify.error('No configuration file for page.', null);
    }
});
</script>