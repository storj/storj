// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-row class="ma-0">
        <v-col v-if="!smAndDown" cols="6">
            <h4 class="font-weight-bold mb-4">Free Trial</h4>
            <v-btn
                block
                disabled
                color="default"
            >
                {{ freeTrialButtonLabel }}
            </v-btn>
            <v-sheet class="my-2">
                <InfoBullet title="Projects" :info="freeProjects" />
                <InfoBullet title="Storage" :info="`${freeUsageValue(user.projectStorageLimit)} limit`" />
                <InfoBullet title="Download" :info="`${freeUsageValue(user.projectBandwidthLimit)} limit`" />
                <InfoBullet title="Segments" :info="`${user.projectSegmentLimit.toLocaleString()} segments limit`" />
                <InfoBullet title="Link Sharing" info="Link sharing with Storj domain" />
                <InfoBullet title="Single User" info="Project can't be shared" />
            </v-sheet>
        </v-col>
        <v-col :cols="smAndDown ? 12 : '6'">
            <h4 class="font-weight-bold mb-4">Pro Account</h4>
            <v-btn
                class="mb-1"
                block
                :loading="loading"
                :append-icon="ArrowRight"
                @click="emit('upgrade')"
            >
                Upgrade
            </v-btn>
            <v-sheet class="my-2">
                <InfoBullet is-pro title="Projects" :info="projectsInfo" />
                <InfoBullet is-pro :title="storagePrice" :info="storagePriceInfo" />
                <InfoBullet is-pro :title="downloadPrice" :info="downloadInfo">
                    <template v-if="downloadMoreInfo" #moreInfo>
                        <p>{{ downloadMoreInfo }}</p>
                    </template>
                </InfoBullet>
                <InfoBullet is-pro title="Segments" :info="segmentInfo">
                    <template #moreInfo>
                        Read more about segment fees in the
                        <a
                            class="link"
                            href="https://docs.storj.io/dcs/pricing#per-segment-fee"
                            target="_blank"
                            rel="noopener noreferrer"
                        >
                            documentation
                        </a>
                    </template>
                </InfoBullet>
                <InfoBullet is-pro title="Secure Custom Domains (HTTPS)" info="Link sharing with your domain" />
                <InfoBullet is-pro title="Team" info="Share projects and collaborate" />
            </v-sheet>
        </v-col>
    </v-row>
</template>

<script setup lang="ts">
import { computed, onBeforeMount, ref } from 'vue';
import { VBtn, VCol, VRow, VSheet } from 'vuetify/components';
import { useDisplay } from 'vuetify';
import { ArrowRight } from 'lucide-vue-next';

import { useUsersStore } from '@/store/modules/usersStore';
import { useNotify } from '@/composables/useNotify';
import { usePreCheck } from '@/composables/usePreCheck';
import { User } from '@/types/users';
import { Size } from '@/utils/bytesSize';
import { useConfigStore } from '@/store/modules/configStore';
import { CENTS_MB_TO_DOLLARS_GB_SHIFT, decimalShift, formatPrice } from '@/utils/strings';

import InfoBullet from '@/components/dialogs/upgradeAccountFlow/InfoBullet.vue';

const configStore = useConfigStore();
const usersStore = useUsersStore();
const notify = useNotify();
const { smAndDown } = useDisplay();
const { isExpired, expirationInfo } = usePreCheck();

defineProps<{
    loading: boolean;
}>();

const emit = defineEmits<{
    upgrade: [];
}>();

const storagePrice = ref<string>('Storage');
const storagePriceInfo = ref<string>('');
const segmentInfo = ref<string>('');
const downloadInfo = ref<string>('');
const projectsInfo = ref<string>('3 projects + more on request');
const downloadPrice = ref<string>('Download');
const downloadMoreInfo = ref<string>('');

/**
 * Returns free trial button label based on expiration status.
 */
const freeTrialButtonLabel = computed<string>(() => {
    if (isExpired.value) return 'Trial Expired';

    return `${expirationInfo.value.days} day${expirationInfo.value.days !== 1 ? 's' : ''} remaining`;
});

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
    const storage = formatPrice(decimalShift(configStore.state.config.storageMBMonthCents, CENTS_MB_TO_DOLLARS_GB_SHIFT));
    const egress = formatPrice(decimalShift(configStore.state.config.egressMBCents, CENTS_MB_TO_DOLLARS_GB_SHIFT));
    const segment = formatPrice(decimalShift(configStore.state.config.segmentMonthCents, 2));
    storagePriceInfo.value = `${storage} per GB-month`;
    downloadInfo.value = `${egress} per GB`;
    segmentInfo.value = `${segment} per segment-month`;
    try {
        const partner = usersStore.state.user.partner;
        const config = (await import('@/configs/upgradeConfig.json')).default;
        if (partner && config[partner]) {
            if (config[partner].storagePriceInfo) {
                storagePriceInfo.value = config[partner].storagePriceInfo;
            }

            if (config[partner].downloadInfo) {
                downloadInfo.value = config[partner].downloadInfo;
            }

            if (config[partner].downloadMoreInfo) {
                downloadMoreInfo.value = config[partner].downloadMoreInfo;
            }
        }
    } catch {
        notify.error('No configuration file for page.', null);
    }
});
</script>
