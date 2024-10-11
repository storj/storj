// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-card title="Credit Card" variant="flat">
        <v-card-text>
            <v-chip color="default" size="small" variant="tonal" class="font-weight-bold mr-2 text-capitalize">{{ card.brand }}</v-chip>
            <v-chip v-if="card.isDefault" color="info" size="small" variant="tonal" class="font-weight-bold">Default</v-chip>
            <v-divider class="my-4" />
            <p>Card Number</p>
            <v-chip color="default" variant="text" class="pl-0 font-weight-bold mt-2">**** **** **** {{ card.last4 }}</v-chip>
            <v-divider class="my-4" />
            <p>Exp. Date</p>
            <v-chip :color="card.isExpired ? 'error' : 'default'" variant="text" class="pl-0 font-weight-bold mt-2">
                <v-row>
                    <v-col cols="12" class="d-flex justify-start align-center pt-2">
                        {{ card.expMonth }}/{{ card.expYear }}
                        <v-chip v-if="card.isExpired" color="error" size="small" variant="tonal" class="font-weight-bold ml-2">Expired</v-chip>
                    </v-col>
                </v-row>
            </v-chip>
            <v-divider class="my-4" />
            <v-row class="ma-0 align-center">
                <v-btn variant="outlined" color="default" size="small" class="mr-2" @click="isEditDefaultCCDialog = true">Edit Default</v-btn>
                <v-btn variant="outlined" color="default" size="small" @click="isRemoveCCDialog = true">Remove</v-btn>
            </v-row>
        </v-card-text>
    </v-card>
    <remove-credit-card-dialog v-model="isRemoveCCDialog" :card="card" @editDefault="isEditDefaultCCDialog = true" />
    <edit-default-credit-card-dialog v-model="isEditDefaultCCDialog" />
</template>

<script setup lang="ts">
import { ref } from 'vue';
import { VBtn, VCard, VCardText, VChip, VDivider, VRow, VCol } from 'vuetify/components';

import { CreditCard } from '@/types/payments';

import RemoveCreditCardDialog from '@/components/dialogs/RemoveCreditCardDialog.vue';
import EditDefaultCreditCardDialog from '@/components/dialogs/EditDefaultCreditCardDialog.vue';

defineProps<{
    card: CreditCard,
}>();

const isRemoveCCDialog = ref<boolean>(false);
const isEditDefaultCCDialog = ref<boolean>(false);
</script>
