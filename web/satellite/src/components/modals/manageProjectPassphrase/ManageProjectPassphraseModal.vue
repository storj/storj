// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <VModal :on-close="closeModal">
        <template #content>
            <div class="modal">
                <LockIcon />
                <ManageOptionsStep
                    v-if="activeStep === ManageProjectPassphraseStep.ManageOptions"
                    :set-create="setCreate"
                    :set-switch="setSwitch"
                    :set-clear="setClear"
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
import { ref } from 'vue';

import { useNotify, useStore } from '@/utils/hooks';
import { APP_STATE_MUTATIONS } from '@/store/mutationConstants';

import VModal from '@/components/common/VModal.vue';
import ManageOptionsStep from '@/components/modals/manageProjectPassphrase/ManageOptionsStep.vue';
import CreateStep from '@/components/modals/manageProjectPassphrase/CreateStep.vue';
import SwitchStep from '@/components/modals/manageProjectPassphrase/SwitchStep.vue';
import ClearStep from '@/components/modals/manageProjectPassphrase/ClearStep.vue';

import LockIcon from '@/../static/images/projectPassphrase/lock.svg';

enum ManageProjectPassphraseStep {
    ManageOptions = 'ManageOptions',
    Create = 'Create',
    Switch = 'Switch',
    Clear = 'Clear',
}

const store = useStore();
const notify = useNotify();

const activeStep = ref<ManageProjectPassphraseStep>(ManageProjectPassphraseStep.ManageOptions);

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
    store.commit(APP_STATE_MUTATIONS.TOGGLE_MANAGE_PROJECT_PASSPHRASE_MODAL_SHOWN);
}
</script>

<style scoped lang="scss">
.modal {
    padding: 40px 60px 68px;

    @media screen and (max-width: 615px) {
        padding: 30px 20px;
    }
}
</style>
