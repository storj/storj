// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <VModal :on-close="closeModal">
        <template #content>
            <div class="modal">
                <div class="modal__header">
                    <AccessEncryptionIcon />
                    <p class="modal__header__title">{{ title }}</p>
                </div>
                <ManageOptionsStep
                    v-if="activeStep === ManageProjectPassphraseStep.ManageOptions"
                    :set-create="setCreate"
                    :set-switch="setSwitch"
                    :set-clear="setClear"
                    :on-cancel="closeModal"
                />
                <CreateStep
                    v-if="activeStep === ManageProjectPassphraseStep.Create"
                    :on-cancel="setManageOptions"
                />
                <SwitchStep
                    v-if="activeStep === ManageProjectPassphraseStep.Switch"
                    :on-cancel="setManageOptions"
                />
                <ClearStep
                    v-if="activeStep === ManageProjectPassphraseStep.Clear"
                    :on-cancel="setManageOptions"
                />
            </div>
        </template>
    </VModal>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue';

import { useNotify } from '@/utils/hooks';
import { MODALS } from '@/utils/constants/appStatePopUps';
import { ManageProjectPassphraseStep } from '@/types/managePassphrase';
import { useAppStore } from '@/store/modules/appStore';

import VModal from '@/components/common/VModal.vue';
import ManageOptionsStep from '@/components/modals/manageProjectPassphrase/ManageOptionsStep.vue';
import CreateStep from '@/components/modals/manageProjectPassphrase/CreateStep.vue';
import SwitchStep from '@/components/modals/manageProjectPassphrase/SwitchStep.vue';
import ClearStep from '@/components/modals/manageProjectPassphrase/ClearStep.vue';

import AccessEncryptionIcon from '@/../static/images/accessGrants/newCreateFlow/accessEncryption.svg';

const appStore = useAppStore();
const notify = useNotify();

/**
 * Returns step from store.
 */
const storedStep = computed((): ManageProjectPassphraseStep | undefined => {
    return appStore.state.managePassphraseStep;
});

const activeStep = ref<ManageProjectPassphraseStep>(storedStep.value || ManageProjectPassphraseStep.ManageOptions);

/**
 * Returns modal title based on active step.
 */
const title = computed((): string => {
    switch (activeStep.value) {
    case ManageProjectPassphraseStep.ManageOptions:
        return 'Manage Passphrase';
    case ManageProjectPassphraseStep.Create:
        return 'Create a new passphrase';
    case ManageProjectPassphraseStep.Switch:
        return 'Switch passphrase';
    case ManageProjectPassphraseStep.Clear:
        return 'Clear my passphrase';
    }

    return '';
});

/**
 * Sets flow to create step.
 */
function setCreate(): void {
    activeStep.value = ManageProjectPassphraseStep.Create;
}

/**
 * Sets flow to switch step.
 */
function setSwitch(): void {
    activeStep.value = ManageProjectPassphraseStep.Switch;
}

/**
 * Sets flow to clear step.
 */
function setClear(): void {
    activeStep.value = ManageProjectPassphraseStep.Clear;
}

/**
 * Sets flow to manage options step.
 */
function setManageOptions(): void {
    activeStep.value = ManageProjectPassphraseStep.ManageOptions;
}

/**
 * Closes modal.
 */
function closeModal(): void {
    appStore.updateActiveModal(MODALS.manageProjectPassphrase);
}

onMounted(() => {
    appStore.setManagePassphraseStep(undefined);
});
</script>

<style scoped lang="scss">
.modal {
    padding: 32px;

    @media screen and (max-width: 615px) {
        padding: 30px 20px;
    }

    &__header {
        display: flex;
        align-items: center;
        padding-bottom: 16px;
        margin-bottom: 16px;
        border-bottom: 1px solid var(--c-grey-2);

        &__title {
            font-family: 'font_bold', sans-serif;
            font-size: 24px;
            line-height: 31px;
            color: var(--c-grey-8);
            margin-left: 16px;
            letter-spacing: -0.02em;
            text-align: left;
        }
    }
}
</style>
