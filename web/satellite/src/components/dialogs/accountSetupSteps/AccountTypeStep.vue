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
                    <div class="h-100 d-flex flex-column justify-space-between pa-8">
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
                    <div class="h-100 d-flex flex-column justify-space-between pa-8">
                        <h3 class="font-weight-black mb-1">Pro Account</h3>
                        <p class="mb-2 text-body-2">
                            Only pay for what you use.
                        </p>

                        <h2 class="font-weight-black"><span class="text-high-emphasis text-body-1 font-weight-bold">Pay as you go</span></h2>
                        <p class="text-medium-emphasis text-caption">no minimum, billed monthly.</p>

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
                                Storage $0.004 per GB per month
                            </p>

                            <v-divider />

                            <p class="text-body-2 my-2">
                                <v-icon :icon="Check" size="14" class="mr-2" />
                                Download bandwidth $0.007 per GB
                            </p>

                            <v-divider />

                            <p class="text-body-2 my-2">
                                <v-icon :icon="Check" size="14" class="mr-2" />
                                Per-segment fee of $0.0000088
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
                <v-btn variant="text" :prepend-icon="ChevronLeft" color="default" block @click="emit('back')">Back</v-btn>
            </v-col>
        </v-row>
    </v-container>
</template>

<script setup lang="ts">
import { VBtn, VCard, VCol, VContainer, VDivider, VIcon, VRow } from 'vuetify/components';
import { ArrowRight, Check, ChevronLeft } from 'lucide-vue-next';

import IconStorjLogo from '@/components/icons/IconStorjLogo.vue';

const emit = defineEmits<{
    freeClick: [];
    proClick: [];
    back: [];
}>();
</script>
