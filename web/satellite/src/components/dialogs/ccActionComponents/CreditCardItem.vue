// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-card border class="mb-6 py-2" variant="flat">
        <v-card-item>
            <v-row class="align-center justify-space-between ma-0">
                <v-col class="pa-0">
                    <v-row class="align-center ma-0">
                        <v-chip rounded class="text-capitalize font-weight-bold">{{ card.brand }}</v-chip>
                        <v-chip v-if="card.isDefault" color="info" rounded class="text-capitalize font-weight-bold ml-2">Default</v-chip>
                    </v-row>
                    <v-card-text class="font-weight-bold px-0">**** **** **** {{ card.last4 }}</v-card-text>
                    <v-card-text class="font-weight-bold pa-0">{{ card.expMonth }}/{{ card.expYear }}</v-card-text>
                </v-col>
                <v-radio v-if="selectable" class="flex-0-0" :value="card.id" color="primary" />
            </v-row>
        </v-card-item>
    </v-card>
</template>

<script setup lang="ts">
import {
    VCol,
    VRow,
    VCard,
    VCardItem,
    VCardText,
    VChip,
    VRadio,
} from 'vuetify/components';

import { CreditCard } from '@/types/payments';

withDefaults(defineProps<{
    card?: CreditCard
    selectable?: boolean
}>(), {
    card: () => new CreditCard(),
    selectable: false,
});
</script>
