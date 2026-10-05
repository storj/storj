// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-dialog
        v-model="model"
        transition="fade-transition"
        width="600"
    >
        <v-card
            rounded="xlg"
            :title="step === Steps.Form ? 'Update Invoice Payment' : 'Audit'"
            :subtitle="step === Steps.Form ? 'Set how this account\'s invoices are paid' : 'Enter a reason for this change'"
        >
            <template #append>
                <v-btn
                    :icon="X" :disabled="isLoading"
                    variant="text" size="small" color="default" @click="model = false"
                />
            </template>

            <v-window v-model="step" :touch="false" class="pa-6">
                <v-window-item :value="Steps.Form">
                    <v-form v-model="formValid" :disabled="isLoading" @submit.prevent="onPrimaryAction">
                        <button type="submit" hidden />
                        <v-row>
                            <v-col cols="12">
                                <v-select
                                    v-model="paymentMethods"
                                    :items="paymentMethodItems"
                                    label="Payment methods"
                                    hint="Stripe payment method types. Leave empty to let Stripe choose."
                                    persistent-hint
                                    multiple
                                    chips
                                    closable-chips
                                    variant="solo-filled"
                                    flat
                                />
                            </v-col>
                            <v-col cols="12">
                                <v-select
                                    v-model="collectionMethod"
                                    :items="collectionMethods"
                                    label="Collection method"
                                    hide-details="auto"
                                    variant="solo-filled"
                                    flat
                                />
                            </v-col>
                            <v-col v-if="collectionMethod === CollectionMethod.SendInvoice" cols="12">
                                <v-number-input
                                    v-model="daysUntilDue"
                                    :rules="[RequiredRule, DaysUntilDueRule]"
                                    :min="1"
                                    label="Days until due"
                                    control-variant="stacked"
                                    hide-details="auto"
                                    variant="solo-filled"
                                    flat
                                />
                            </v-col>
                        </v-row>
                        <v-alert
                            v-if="isClearing"
                            class="mt-4"
                            type="warning"
                            variant="tonal"
                            text="This clears the entitlement. Stripe will choose how new invoices are paid."
                        />
                    </v-form>
                </v-window-item>
                <v-window-item :value="Steps.Reason">
                    <v-form :model-value="!!reason" :disabled="isLoading" @submit.prevent="onPrimaryAction">
                        <button type="submit" hidden />
                        <v-textarea
                            v-model="reason"
                            :rules="[RequiredRule]"
                            label="Reason"
                            placeholder="Enter reason for this change"
                            hide-details="auto"
                            variant="solo-filled"
                            autofocus
                            flat
                        />
                    </v-form>
                </v-window-item>
            </v-window>

            <v-card-actions class="pa-6">
                <v-row>
                    <v-col>
                        <v-btn
                            variant="outlined"
                            color="default"
                            block
                            :disabled="isLoading"
                            @click="onSecondaryAction"
                        >
                            {{ secondaryActionText }}
                        </v-btn>
                    </v-col>
                    <v-col>
                        <v-btn
                            color="primary"
                            variant="flat"
                            :disabled="submitDisabled"
                            :loading="isLoading"
                            block
                            @click="onPrimaryAction"
                        >
                            {{ primaryActionText }}
                        </v-btn>
                    </v-col>
                </v-row>
            </v-card-actions>
        </v-card>
    </v-dialog>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue';
import { X } from 'lucide-vue-next';
import {
    VAlert,
    VBtn,
    VCard,
    VCardActions,
    VCol,
    VDialog,
    VForm,
    VNumberInput,
    VRow,
    VSelect,
    VTextarea,
    VWindow,
    VWindowItem,
} from 'vuetify/components';

import { SetInvoicePaymentRequest, UserEntitlements } from '@/api/client.gen';
import { useLoading } from '@/composables/useLoading';
import { useNotify } from '@/composables/useNotify';
import { RequiredRule } from '@/types/common';
import { CollectionMethod } from '@/types/billing';
import { useAppStore } from '@/store/app';
import { useBillingStore } from '@/store/billing';

enum Steps {
    Form = 1,
    Reason = 2,
}

const props = defineProps<{
    userId: string;
    entitlements: UserEntitlements;
}>();

const model = defineModel<boolean>({ required: true });

const emit = defineEmits<{
    updated: [entitlements: UserEntitlements];
}>();

const appStore = useAppStore();
const billingStore = useBillingStore();
const notify = useNotify();
const { isLoading, withLoading } = useLoading();

const collectionMethods = [
    { title: 'Stripe default', value: CollectionMethod.Default },
    { title: 'Charge automatically', value: CollectionMethod.ChargeAutomatically },
    { title: 'Send invoice', value: CollectionMethod.SendInvoice },
];

/**
 * The payment methods the satellite allows, plus any the account already has, so that a
 * method no longer allowed is still shown and can be removed.
 */
const paymentMethodItems = computed<string[]>(() => {
    const items = [...(appStore.state.settings.admin.invoicePaymentMethods ?? [])];
    for (const m of props.entitlements.invoicePaymentMethods ?? []) {
        if (!items.includes(m)) items.push(m);
    }
    return items;
});

const step = ref<Steps>(Steps.Form);
const formValid = ref<boolean>(false);
const reason = ref<string>('');
const paymentMethods = ref<string[]>([]);
const collectionMethod = ref<CollectionMethod>(CollectionMethod.Default);
const daysUntilDue = ref<number | null>(null);

function DaysUntilDueRule(value: unknown): string | boolean {
    const num = Number(value);
    return (Number.isInteger(num) && num > 0) || 'Must be a whole number greater than 0';
}

/**
 * The request as the server will store it: methods are normalized the same way the
 * satellite does, and days until due only goes with send_invoice.
 */
const request = computed<Omit<SetInvoicePaymentRequest, 'reason'>>(() => {
    const methods: string[] = [];
    for (const m of paymentMethods.value) {
        const normalized = m.trim().toLowerCase();
        if (normalized && !methods.includes(normalized)) methods.push(normalized);
    }
    const isSendInvoice = collectionMethod.value === CollectionMethod.SendInvoice;
    return {
        paymentMethods: methods,
        collectionMethod: collectionMethod.value,
        daysUntilDue: isSendInvoice ? Number(daysUntilDue.value ?? 0) : 0,
    };
});

const hasChanged = computed<boolean>(() => {
    const current = request.value;
    const original = props.entitlements;
    const originalMethods = original.invoicePaymentMethods ?? [];

    return current.collectionMethod !== original.invoiceCollectionMethod ||
        current.daysUntilDue !== original.invoiceDaysUntilDue ||
        current.paymentMethods?.length !== originalMethods.length ||
        current.paymentMethods.some((m, i) => m !== originalMethods[i]);
});

const isClearing = computed<boolean>(() => {
    const r = request.value;
    return hasChanged.value && !r.paymentMethods?.length && !r.collectionMethod;
});

const secondaryActionText = computed(() => (step.value === Steps.Form ? 'Cancel' : 'Back'));
const primaryActionText = computed(() => (step.value === Steps.Form ? 'Continue' : 'Submit'));
const submitDisabled = computed(() => (step.value === Steps.Form ? !hasChanged.value || !formValid.value : !reason.value));

function onPrimaryAction() {
    if (submitDisabled.value) return;
    if (step.value === Steps.Form) {
        step.value = Steps.Reason;
    } else {
        update();
    }
}

function onSecondaryAction() {
    if (step.value === Steps.Form) {
        model.value = false;
    } else {
        step.value = Steps.Form;
    }
}

function update() {
    withLoading(async () => {
        const req = new SetInvoicePaymentRequest();
        Object.assign(req, request.value);
        req.reason = reason.value;

        try {
            const updated = await billingStore.setInvoicePayment(props.userId, req);
            emit('updated', updated);
            notify.success('Invoice payment updated successfully.');
            model.value = false;
        } catch (error) {
            notify.error(`Failed to update invoice payment. ${error.message}`);
        }
    });
}

function resetForm() {
    step.value = Steps.Form;
    reason.value = '';
    paymentMethods.value = [...(props.entitlements.invoicePaymentMethods ?? [])];
    collectionMethod.value = props.entitlements.invoiceCollectionMethod as CollectionMethod;
    daysUntilDue.value = props.entitlements.invoiceDaysUntilDue || null;
}

watch(model, (newValue) => {
    if (newValue) resetForm();
});
</script>
