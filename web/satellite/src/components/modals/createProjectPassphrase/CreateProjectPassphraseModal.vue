// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <VModal :on-close="closeModal">
        <template #content>
            <div class="modal">
                <SelectPassphraseModeStep
                    v-if="activeStep === CreateProjectPassphraseStep.SelectMode"
                    :is-generate="selectedOption === CreatePassphraseOption.Generate"
                    :set-generate="() => setOption(CreatePassphraseOption.Generate)"
                    :set-enter="() => setOption(CreatePassphraseOption.Enter)"
                    :on-cancel="onCancelOrBack"
                    :on-continue="onContinue"
                />
                <PassphraseGeneratedStep
                    v-if="activeStep === CreateProjectPassphraseStep.PassphraseGenerated"
                    :on-back="onCancelOrBack"
                    :on-continue="onContinue"
                    :passphrase="generatedPassphrase"
                    name="storj"
                />
                <EnterPassphraseStep
                    v-if="activeStep === CreateProjectPassphraseStep.EnterPassphrase"
                    :set-passphrase="setPassphrase"
                    :passphrase="passphrase"
                    :on-back="onCancelOrBack"
                    :on-continue="onContinue"
                />
                <SuccessStep
                    v-if="activeStep === CreateProjectPassphraseStep.Success"
                    :on-continue="onContinue"
                />
            </div>
        </template>
    </VModal>
</template>

<script setup lang="ts">
import { ref } from 'vue';
import { generateMnemonic } from 'bip39-english';
import { useRoute, useRouter } from 'vue-router';

import { useNotify } from '@/utils/hooks';
import { RouteConfig } from '@/types/router';
import { EdgeCredentials } from '@/types/accessGrants';
import { useAppStore } from '@/store/modules/appStore';
import { useBucketsStore } from '@/store/modules/bucketsStore';
import { AnalyticsErrorEventSource } from '@/utils/constants/analyticsEventNames';

import VModal from '@/components/common/VModal.vue';
import SelectPassphraseModeStep from '@/components/modals/createProjectPassphrase/SelectPassphraseModeStep.vue';
import PassphraseGeneratedStep from '@/components/modals/createProjectPassphrase/PassphraseGeneratedStep.vue';
import EnterPassphraseStep from '@/components/modals/createProjectPassphrase/EnterPassphraseStep.vue';
import SuccessStep from '@/components/modals/createProjectPassphrase/SuccessStep.vue';

enum CreateProjectPassphraseStep {
    SelectMode = 'SelectMode',
    PassphraseGenerated = 'PassphraseGenerated',
    EnterPassphrase = 'EnterPassphrase',
    Success = 'Success',
}

enum CreatePassphraseOption {
    Generate = 'Generate',
    Enter = 'Enter',
}

const bucketsStore = useBucketsStore();
const appStore = useAppStore();
const notify = useNotify();
const router = useRouter();
const route = useRoute();

const generatedPassphrase = generateMnemonic();

const selectedOption = ref<CreatePassphraseOption>(CreatePassphraseOption.Generate);
const activeStep = ref<CreateProjectPassphraseStep>(CreateProjectPassphraseStep.SelectMode);
const passphrase = ref<string>('');

/**
 * Sets passphrase input value to local variable.
 * Resets error is present.
 * @param value
 */
function setPassphrase(value: string): void {
    passphrase.value = value;
}

/**
 * Sets create passphrase option (generated or entered).
 * @param option
 */
function setOption(option: CreatePassphraseOption): void {
    selectedOption.value = option;
}

/**
 * Closes modal.
 */
function closeModal(): void {
    appStore.removeActiveModal();
}

/**
 * Holds on continue button click logic.
 * Navigates further through flow.
 */
async function onContinue(): Promise<void> {
    if (activeStep.value === CreateProjectPassphraseStep.SelectMode) {
        if (selectedOption.value === CreatePassphraseOption.Generate) {
            if (passphrase.value) {
                passphrase.value = '';
            }

            passphrase.value = generatedPassphrase;
            activeStep.value = CreateProjectPassphraseStep.PassphraseGenerated;
            return;
        }

        if (selectedOption.value === CreatePassphraseOption.Enter) {
            if (passphrase.value) {
                passphrase.value = '';
            }
            activeStep.value = CreateProjectPassphraseStep.EnterPassphrase;
            return;
        }
    }

    if (
        activeStep.value === CreateProjectPassphraseStep.PassphraseGenerated ||
        activeStep.value === CreateProjectPassphraseStep.EnterPassphrase
    ) {
        if (!passphrase.value) {
            notify.error('Passphrase can\'t be empty', AnalyticsErrorEventSource.CREATE_PROJECT_PASSPHRASE_MODAL);
            return;
        }

        bucketsStore.setEdgeCredentials(new EdgeCredentials());
        bucketsStore.setPassphrase(passphrase.value);
        bucketsStore.setPromptForPassphrase(false);

        activeStep.value = CreateProjectPassphraseStep.Success;

        return;
    }

    if (activeStep.value === CreateProjectPassphraseStep.Success) {
        if (route.name === RouteConfig.OverviewStep.name) {
            router.push(RouteConfig.ProjectDashboard.path);
        }

        closeModal();
    }
}

/**
 * Holds on cancel/back button click logic.
 * Navigates backwards through flow.
 */
function onCancelOrBack(): void {
    if (activeStep.value === CreateProjectPassphraseStep.SelectMode) {
        closeModal();
        return;
    }

    if (
        activeStep.value === CreateProjectPassphraseStep.PassphraseGenerated ||
        activeStep.value === CreateProjectPassphraseStep.EnterPassphrase
    ) {
        passphrase.value = '';
        activeStep.value = CreateProjectPassphraseStep.SelectMode;
    }
}
</script>

<style scoped lang="scss">
.modal {
    padding: 32px;
    font-family: 'font_regular', sans-serif;

    @media screen and (width <= 615px) {
        padding: 30px 20px;
    }
}
</style>
