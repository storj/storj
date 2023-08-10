// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-card title="Credit Card" variant="flat" :border="true" rounded="xlg">
        <v-card-text>
            <v-chip rounded color="default" variant="tonal" class="font-weight-bold mr-2 text-capitalize">{{ card.brand }}</v-chip>
            <v-chip v-if="card.isDefault" rounded color="primary" variant="tonal" class="font-weight-bold">Default</v-chip>
            <v-divider class="my-4" />
            <p>Card Number</p>
            <v-chip rounded color="default" variant="text" class="pl-0 font-weight-bold mt-2">**** **** **** {{ card.last4 }}</v-chip>
            <v-divider class="my-4" />
            <p>Exp. Date</p>
            <v-chip rounded color="default" variant="text" class="pl-0 font-weight-bold mt-2">
                {{ card.expMonth }}/{{ card.expYear }}
            </v-chip>
            <v-divider class="my-4" />
            <v-btn variant="outlined" color="default" size="small" class="mr-2">Remove</v-btn>
        </v-card-text>
    </v-card>
</template>

<script setup lang="ts">
import { VBtn, VCard, VCardText, VChip, VDivider } from 'vuetify/components';

import { useUsersStore } from '@/store/modules/usersStore';
import { CreditCard } from '@/types/payments';

const usersStore = useUsersStore();

const props = defineProps<{
    card: CreditCard,
}>();
</script>