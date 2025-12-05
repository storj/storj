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
            <PricingPlans
                @free-click="emit('freeClick')"
                @pro-click="emit('proClick')"
                @pkg-click="emit('pkgClick')"
            />
        </v-row>

        <v-row justify="center" class="mt-8">
            <v-col cols="6" sm="4" md="3" lg="2">
                <v-btn variant="text" class="text-medium-emphasis" :prepend-icon="ChevronLeft" color="default" block @click="emit('back')">Back</v-btn>
            </v-col>
        </v-row>
    </v-container>
</template>

<script setup lang="ts">
import { VBtn, VCol, VContainer, VRow } from 'vuetify/components';
import { ChevronLeft } from 'lucide-vue-next';

import IconStorjLogo from '@/components/icons/IconStorjLogo.vue';
import PricingPlans from '@/components/dialogs/upgradeAccountFlow/PricingPlans.vue';

const emit = defineEmits<{
    freeClick: [];
    proClick: [];
    pkgClick: [];
    back: [];
}>();
</script>
