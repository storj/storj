// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-container>
        <v-row justify="center">
            <v-col class="text-center py-4">
                <icon-storj-logo height="50" width="50" class="rounded-xlg bg-background pa-2 border" />
                <p class="text-overline mt-2 mb-1">
                    Account Type
                </p>
                <h2>Choose your account type</h2>
            </v-col>
        </v-row>

        <v-row justify="center">
            <v-col cols="12" sm="8" md="6" lg="4">
                <v-card variant="outlined" rounded="xlg" class="h-100">
                    <div class="h-100 d-flex flex-column justify-space-between pa-6 pa-sm-8">
                        <h3 class="font-weight-black mb-1">Free Trial</h3>
                        <p class="mb-2 text-body-2">Great to start using Storj.</p>

                        <h2 class="font-weight-black"><span class="text-high-emphasis text-body-1 font-weight-bold">Free</span></h2>
                        <p class="text-medium-emphasis text-caption">30 days trial, no card needed.</p>

                        <v-btn
                            id="free-plan"
                            class="mt-4 mb-4"
                            color="primary"
                            variant="flat"
                            @click="emit('freeClick')"
                        >
                            <template #append>
                                <v-icon :icon="ArrowRight" />
                            </template>
                            Start Free Trial
                        </v-btn>

                        <div class="text-left">
                            <p class="text-body-2 my-2"><v-icon :icon="Check" class="mr-2" size="14" />1 project</p>
                            <v-divider />
                            <p class="text-body-2 my-2"><v-icon :icon="Check" class="mr-2" size="14" />25GB storage included</p>
                            <v-divider />
                            <p class="text-body-2 my-2"><v-icon :icon="Check" class="mr-2" size="14" />25GB download included</p>
                            <v-divider />
                            <p class="text-body-2 my-2"><v-icon :icon="Check" class="mr-2" size="14" />10,000 segments included</p>
                            <v-divider />
                            <p class="text-body-2 my-2"><v-icon :icon="Check" class="mr-2" size="14" />Fixed monthly usage limits</p>
                            <v-divider />
                            <p class="text-body-2 my-2"><v-icon :icon="Check" class="mr-2" size="14" />Unlimited team members</p>
                            <v-divider />
                            <p class="text-body-2 mt-2"><v-icon :icon="Check" class="mr-2" size="14" />Share links with Storj domain</p>
                        </div>
                    </div>
                </v-card>
            </v-col>

            <v-col cols="12" sm="8" md="6" lg="4">
                <v-card variant="outlined" rounded="xlg" class="h-100">
                    <div class="h-100 d-flex flex-column justify-space-between pa-6 pa-sm-8">
                        <h3 class="font-weight-black mb-1">Activate your account</h3>
                        <p class="mb-2 text-body-2">
                            Only pay for what you use.
                        </p>

                        <h2 class="font-weight-black"><span class="text-high-emphasis text-body-1 font-weight-bold">Pay as you go</span></h2>
                        <p class="text-medium-emphasis text-caption">
                            <template v-if="minimumCharge.priorNoticeEnabled">
                                A <a href="https://storj.dev/dcs/pricing#minimum-monthly-billing" target="_blank">minimum monthly usage fee</a>
                                of {{ minimumCharge.amount }} {{ isAfterStartDate ? 'applies' : 'will apply' }} starting on {{ minimumCharge.shortStartDateStr }}.
                            </template>
                            <template v-else-if="minimumCharge.isEnabled">
                                A <a href="https://storj.dev/dcs/pricing#minimum-monthly-billing" target="_blank">minimum monthly usage fee</a>
                                of {{ minimumCharge.amount }} applies.
                            </template>
                            <template v-else>
                                No minimum, billed monthly.
                            </template>
                        </p>

                        <v-btn
                            variant="outlined"
                            color="text-secondary"
                            class="mt-4 mb-4"
                            @click="emit('proClick')"
                        >
                            Start Pro Account
                            <template #append>
                                <v-icon :icon="ArrowRight" />
                            </template>
                        </v-btn>

                        <div class="text-left">
                            <p class="text-body-2 my-2">
                                <v-icon :icon="Check" size="14" class="mr-2" />
                                3 projects (+ more on request)
                            </p>

                            <v-divider />

                            <p class="text-body-2 my-2">
                                <v-icon :icon="Check" size="14" class="mr-2" />
                                Storage as low as {{ storageLabel }}
                            </p>

                            <v-divider />

                            <p class="text-body-2 my-2">
                                <v-icon :icon="Check" size="14" class="mr-2" />
                                {{ downloadLabel }}
                                <v-tooltip v-if="downloadInfo" top max-width="300px">
                                    <template #activator="{ props }">
                                        <span v-bind="props" class="ml-1">
                                            <v-icon :icon="Info" size="14" />
                                        </span>
                                    </template>
                                    <span>{{ downloadInfo }}</span>
                                </v-tooltip>
                            </p>

                            <v-divider />

                            <p class="text-body-2 my-2">
                                <v-icon :icon="Check" size="14" class="mr-2" />
                                Per-segment fee of {{ segmentPrice }}
                            </p>

                            <v-divider />

                            <p class="text-body-2 my-2">
                                <v-icon :icon="Check" size="14" class="mr-2" />
                                Set your own usage limits
                            </p>

                            <v-divider />

                            <p class="text-body-2 my-2">
                                <v-icon :icon="Check" size="14" class="mr-2" />
                                Unlimited team members
                            </p>

                            <v-divider />

                            <p class="text-body-2 mt-2">
                                <v-icon :icon="Check" size="14" class="mr-2" />
                                Custom domain support
                            </p>
                        </div>
                    </div>
                </v-card>
            </v-col>
        </v-row>

        <v-row justify="center" class="mt-8">
            <v-col cols="6" sm="4" md="3" lg="2">
                <v-btn variant="text" class="text-medium-emphasis" :prepend-icon="ChevronLeft" color="default" block @click="emit('back')">Back</v-btn>
            </v-col>
        </v-row>
    </v-container>
</template>

<script setup lang="ts">
import { VBtn, VCard, VCol, VContainer, VDivider, VIcon, VRow, VTooltip } from 'vuetify/components';
import { ArrowRight, Check, ChevronLeft, Info } from 'lucide-vue-next';
import { computed, onBeforeMount, ref } from 'vue';

import { useBillingStore } from '@/store/modules/billingStore';
import { useConfigStore } from '@/store/modules/configStore';
import { useUsersStore } from '@/store/modules/usersStore';

import IconStorjLogo from '@/components/icons/IconStorjLogo.vue';

const billingStore = useBillingStore();
const configStore = useConfigStore();
const usersStore = useUsersStore();

const emit = defineEmits<{
    freeClick: [];
    proClick: [];
    back: [];
}>();

const storagePrice = computed(() => billingStore.storagePrice);

const egressPrice = computed(() => billingStore.egressPrice);

const segmentPrice = computed(() => billingStore.segmentPrice);

const minimumCharge = computed(() => configStore.minimumCharge);

const isAfterStartDate = computed(() => {
    return minimumCharge.value.startDate && new Date() >= minimumCharge.value.startDate;
});

const storageLabel = ref<string>(storagePrice.value);
const downloadLabel = ref<string>(`Download bandwidth as low as ${egressPrice.value}`);
const downloadInfo = ref<string>('');

onBeforeMount(async () => {
    try {
        const partner = usersStore.state.user.partner;
        const config = (await import('@/configs/upgradeConfig.json')).default;
        if (partner && config[partner]) {
            if (config[partner].storagePriceInfo) {
                storageLabel.value = config[partner].storagePriceInfo;
            }

            if (config[partner].downloadInfo) {
                downloadLabel.value = config[partner].downloadInfo;
            }

            if (config[partner].downloadMoreInfo) {
                downloadInfo.value = config[partner].downloadMoreInfo;
            }
        }
    } catch {
        // ignore error.
    }
});
</script>
