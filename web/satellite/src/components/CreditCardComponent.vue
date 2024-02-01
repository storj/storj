// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-card title="Credit Card" variant="flat" border rounded="xlg">
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
import { VBtn, VCard, VCardText, VChip, VDivider, VRow } from 'vuetify/components';

import { useUsersStore } from '@/store/modules/usersStore';
import { CreditCard } from '@/types/payments';

import RemoveCreditCardDialog from '@/components/dialogs/RemoveCreditCardDialog.vue';
import EditDefaultCreditCardDialog from '@/components/dialogs/EditDefaultCreditCardDialog.vue';

const usersStore = useUsersStore();

const props = defineProps<{
    card: CreditCard,
}>();

const isRemoveCCDialog = ref<boolean>(false);
const isEditDefaultCCDialog = ref<boolean>(false);
</script>
