// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <OptOutConfirmationDialog v-model="isConfirmDialogShown" @confirm="onConfirmOptOut" />
    <PlanConfirmedDialog v-model="isPlanConfirmedDialogShown" @confirm="onPlanConfirmed" />

    <v-dialog
        v-model="model"
        fullscreen
        persistent
        transition="fade-transition"
    >
        <v-sheet color="surface" height="100%" class="d-flex align-center justify-center pa-6">
            <div :style="{ maxWidth: containerMaxWidth, width: '100%' }" class="text-center mx-auto">
                <!-- Logo -->
                <div class="mb-5">
                    <icon-storj-logo
                        height="50"
                        width="50"
                        class="rounded-xlg bg-background pa-2 border"
                    />
                </div>

                <!-- Effective date -->
                <p class="text-label-medium text-medium-emphasis mb-1">
                    EFFECTIVE JULY 1, 2026
                </p>

                <!-- Headline -->
                <h2 class="text-headline-medium font-weight-bold mb-2">
                    Your storage plan is changing.
                </h2>

                <!-- Subtitle -->
                <p class="text-body-medium text-medium-emphasis mb-6">
                    {{ generalDescription }}
                </p>
                <!-- Pricing cards -->
                <div class="d-flex flex-wrap justify-center ga-4 mb-6">
                    <v-card
                        v-for="card in cards"
                        :key="card.label"
                        variant="outlined"
                        rounded="lg"
                        class="text-left flex-grow-1"
                        :style="cards.length > 1 ? 'flex-basis: 0; min-width: 280px;' : 'max-width: 100%;'"
                    >
                        <v-card-text class="pa-6">
                            <p class="font-weight-bold text-body-large mb-1">{{ card.label }}</p>
                            <p class="text-title-large font-weight-bold text-primary mb-4">{{ card.planName }}</p>
                            <div class="d-flex flex-column ga-3">
                                <div v-for="feature in card.features" :key="feature" class="d-flex align-center ga-2">
                                    <v-icon :icon="Check" size="16" />
                                    <span class="text-body-medium">{{ feature }}</span>
                                </div>
                            </div>
                        </v-card-text>
                    </v-card>
                </div>

                <!-- Footer text -->
                <p class="text-body-medium text-medium-emphasis mb-6">
                    For a full breakdown of pricing details and fees, visit the <a class="link" href="https://storj.dev/dcs/pricing" target="_blank" rel="noopener noreferrer">Pricing Details</a>.<br>
                    Clicking Accept &amp; Continue confirms you've reviewed these changes.<br>
                    Have questions? Read the
                    <a href="https://storj.io/pricing" target="_blank" rel="noopener noreferrer" class="text-primary">pricing FAQ</a>
                    or
                    <a href="https://storj.io/contact-sales" target="_blank" rel="noopener noreferrer" class="text-primary">contact support</a>.
                </p>

                <!-- Actions -->
                <div class="d-flex align-center justify-center ga-3 flex-wrap">
                    <v-btn
                        variant="outlined"
                        color="default"
                        size="large"
                        :loading="isOptingOut"
                        :disabled="isOptingIn"
                        @click="onDecline"
                    >
                        Opt-out and leave
                    </v-btn>
                    <v-btn
                        color="primary"
                        size="large"
                        :append-icon="ArrowRight"
                        :loading="isOptingIn"
                        :disabled="isOptingOut"
                        @click="onOptIn"
                    >
                        Accept &amp; Continue
                    </v-btn>
                </div>
            </div>
        </v-sheet>
    </v-dialog>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue';
import { VBtn, VCard, VCardText, VDialog, VIcon, VSheet } from 'vuetify/components';
import { ArrowRight, Check } from 'lucide-vue-next';

import { OptInStatus } from '@/types/users';
import { useUsersStore } from '@/store/modules/usersStore';
import { useAppStore } from '@/store/modules/appStore';
import { useNotify } from '@/composables/useNotify';
import {
    cardsForVariant,
    generalPricingOptionsDescription,
    resolvePricingOptInVariant,
    type PricingOptInCard,
} from '@/types/pricingOptIn';

import OptOutConfirmationDialog from '@/components/dialogs/OptOutConfirmationDialog.vue';
import PlanConfirmedDialog from '@/components/dialogs/PlanConfirmedDialog.vue';
import IconStorjLogo from '@/components/icons/IconStorjLogo.vue';

const usersStore = useUsersStore();
const appStore = useAppStore();
const notify = useNotify();

const model = defineModel<boolean>({ required: true });

const isOptingIn = ref(false);
const isOptingOut = ref(false);
const isConfirmDialogShown = ref(false);
const isPlanConfirmedDialogShown = ref(false);

const cards = computed<PricingOptInCard[]>(() => cardsForVariant(resolvePricingOptInVariant()));
const containerMaxWidth = computed<string>(() => cards.value.length > 1 ? '880px' : '680px');
const generalDescription = computed<string>(() => generalPricingOptionsDescription(resolvePricingOptInVariant()));
const currentStatus = computed<OptInStatus>(() => usersStore.state.settings.optInStatus);

async function onOptIn(): Promise<void> {
    if (isOptingIn.value || isOptingOut.value) return;
    isOptingIn.value = true;
    try {
        await usersStore.updateSettings({ optInStatus: OptInStatus.OptedIn });
        isPlanConfirmedDialogShown.value = true;
        // get user to refresh freeze status
        usersStore.getUser().catch(() => { /* ignored */ });
    } catch (error) {
        notify.notifyError(error);
    } finally {
        isOptingIn.value = false;
    }
}

function onPlanConfirmed(): void {
    isPlanConfirmedDialogShown.value = false;
    appStore.togglePricingOptInDialog(false);
}

function onDecline(): void {
    if (isOptingIn.value || isOptingOut.value) return;
    isConfirmDialogShown.value = true;
}

async function onConfirmOptOut(): Promise<void> {
    if (isOptingIn.value || isOptingOut.value) return;
    isOptingOut.value = true;
    try {
        if (currentStatus.value !== OptInStatus.OptedOut) {
            await usersStore.updateSettings({ optInStatus: OptInStatus.OptedOut });
        }

        isConfirmDialogShown.value = false;
        appStore.togglePricingOptInDialog(false);
    } catch (error) {
        notify.notifyError(error);
    } finally {
        isOptingOut.value = false;
    }
}
</script>
