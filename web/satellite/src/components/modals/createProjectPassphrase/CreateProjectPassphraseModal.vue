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
                />
                <PassphraseGeneratedStep
                    v-if="activeStep === CreateProjectPassphraseStep.PassphraseGenerated"
                    :passphrase="passphrase"
                />
                <EnterPassphraseStep
                    v-if="activeStep === CreateProjectPassphraseStep.EnterPassphrase"
                    :set-passphrase="setPassphrase"
                    :enter-error="enterError"
                />
                <SuccessStep v-if="activeStep === CreateProjectPassphraseStep.Success" />
                <div v-if="isCheckVisible" class="modal__save-container" @click="toggleSaved">
                    <div class="modal__save-container__check" :class="{checked: passphraseSaved}">
                        <CheckIcon />
                    </div>
                    <div class="modal__save-container__info">
                        <h2 class="modal__save-container__info__title">
                            Yes I understand and saved the passphrase.
                        </h2>
                        <p class="modal__save-container__info__msg">
                            Check the box to continue.
                        </p>
                    </div>
                </div>
                <div class="modal__buttons">
                    <VButton
                        v-if="activeStep !== CreateProjectPassphraseStep.Success"
                        :label="activeStep === CreateProjectPassphraseStep.SelectMode ? 'Cancel' : 'Back'"
                        width="100%"
                        height="48px"
                        :is-white="true"
                        :on-press="onCancelOrBack"
                    />
                    <VButton
                        label="Continue ->"
                        :width="activeStep === CreateProjectPassphraseStep.Success ? '200px' : '100%'"
                        height="48px"
                        :on-press="onContinue"
                        :is-disabled="continueButtonDisabled"
                    />
                </div>
            </div>
        </template>
    </VModal>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue';
import { generateMnemonic } from 'bip39';

import { useNotify, useStore } from '@/utils/hooks';
import { APP_STATE_MUTATIONS } from '@/store/mutationConstants';
import { ACCESS_GRANTS_ACTIONS } from '@/store/modules/accessGrants';
import { AccessGrant, EdgeCredentials } from '@/types/accessGrants';
import { OBJECTS_ACTIONS, OBJECTS_MUTATIONS } from '@/store/modules/objects';
import { PROJECTS_ACTIONS } from '@/store/modules/projects';
import { MetaUtils } from '@/utils/meta';

import VModal from '@/components/common/VModal.vue';
import VButton from '@/components/common/VButton.vue';
import SelectPassphraseModeStep from '@/components/modals/createProjectPassphrase/SelectPassphraseModeStep.vue';
import PassphraseGeneratedStep from '@/components/modals/createProjectPassphrase/PassphraseGeneratedStep.vue';
import EnterPassphraseStep from '@/components/modals/createProjectPassphrase/EnterPassphraseStep.vue';
import SuccessStep from '@/components/modals/createProjectPassphrase/SuccessStep.vue';

import CheckIcon from '@/../static/images/projectPassphrase/check.svg';

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

const FILE_BROWSER_AG_NAME = 'Web file browser API key';

const store = useStore();
const notify = useNotify();

const selectedOption = ref<CreatePassphraseOption>(CreatePassphraseOption.Generate);
const activeStep = ref<CreateProjectPassphraseStep>(CreateProjectPassphraseStep.SelectMode);
const passphrase = ref<string>('');
const enterError = ref<string>('');
const isLoading = ref<boolean>(false);
const passphraseSaved = ref<boolean>(false);
const worker = ref<Worker | null>(null);

/**
 * Indicates if save passphrase checkbox container is shown.
 */
const isCheckVisible = computed((): boolean => {
    return activeStep.value === CreateProjectPassphraseStep.PassphraseGenerated ||
        activeStep.value === CreateProjectPassphraseStep.EnterPassphrase;
});

/**
 * Indicates if continue button is disabled.
 */
const continueButtonDisabled = computed((): boolean => {
    return (isCheckVisible.value && !passphraseSaved.value) || isLoading.value;
});

/**
 * Returns web file browser api key from vuex state.
 */
const apiKey = computed((): string => {
    return store.state.objectsModule.apiKey;
});

/**
 * Lifecycle hook after initial render.
 * Sets local worker.
 */
onMounted(() => {
    setWorker();
});

/**
 * Sets passphrase input value to local variable.
 * Resets error is present.
 * @param value
 */
function setPassphrase(value: string): void {
    if (enterError.value) {
        enterError.value = '';
    }

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
 * Toggles save passphrase checkbox.
 */
function toggleSaved(): void {
    passphraseSaved.value = !passphraseSaved.value;
}

/**
 * Closes modal.
 */
function closeModal(): void {
    store.commit(APP_STATE_MUTATIONS.TOGGLE_CREATE_PROJECT_PASSPHRASE_MODAL_SHOWN);
}

/**
 * Sets local worker with worker instantiated in store.
 */
function setWorker(): void {
    worker.value = store.state.accessGrantsModule.accessGrantsWebWorker;
    if (worker.value) {
        worker.value.onerror = (error: ErrorEvent) => {
            notify.error(error.message);
        };
    }
}

/**
 * Generates s3 credentials from provided passphrase and stores it in vuex state to be reused.
 */
async function setAccess(): Promise<void> {
    if (!worker.value) {
        notify.error('Worker is not defined');
        return;
    }

    if (!apiKey.value) {
        await store.dispatch(ACCESS_GRANTS_ACTIONS.DELETE_BY_NAME_AND_PROJECT_ID, FILE_BROWSER_AG_NAME);
        const cleanAPIKey: AccessGrant = await store.dispatch(ACCESS_GRANTS_ACTIONS.CREATE, FILE_BROWSER_AG_NAME);
        await store.dispatch(OBJECTS_ACTIONS.SET_API_KEY, cleanAPIKey.secret);
    }

    const now = new Date();
    const inThreeDays = new Date(now.setDate(now.getDate() + 3));

    await worker.value.postMessage({
        'type': 'SetPermission',
        'isDownload': true,
        'isUpload': true,
        'isList': true,
        'isDelete': true,
        'notAfter': inThreeDays.toISOString(),
        'buckets': [],
        'apiKey': apiKey.value,
    });

    const grantEvent: MessageEvent = await new Promise(resolve => {
        if (worker.value) {
            worker.value.onmessage = resolve;
        }
    });
    if (grantEvent.data.error) {
        throw new Error(grantEvent.data.error);
    }

    const salt = await store.dispatch(PROJECTS_ACTIONS.GET_SALT, store.getters.selectedProject.id);
    const satelliteNodeURL: string = MetaUtils.getMetaContent('satellite-nodeurl');

    worker.value.postMessage({
        'type': 'GenerateAccess',
        'apiKey': grantEvent.data.value,
        'passphrase': passphrase.value,
        'salt': salt,
        'satelliteNodeURL': satelliteNodeURL,
    });

    const accessGrantEvent: MessageEvent = await new Promise(resolve => {
        if (worker.value) {
            worker.value.onmessage = resolve;
        }
    });
    if (accessGrantEvent.data.error) {
        throw new Error(accessGrantEvent.data.error);
    }

    const accessGrant = accessGrantEvent.data.value;

    const gatewayCredentials: EdgeCredentials = await store.dispatch(ACCESS_GRANTS_ACTIONS.GET_GATEWAY_CREDENTIALS, { accessGrant });
    await store.dispatch(OBJECTS_ACTIONS.SET_GATEWAY_CREDENTIALS, gatewayCredentials);
    await store.dispatch(OBJECTS_ACTIONS.SET_S3_CLIENT);
    await store.commit(OBJECTS_MUTATIONS.SET_PROMPT_FOR_PASSPHRASE, false);
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

            passphrase.value = generateMnemonic();
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
        if (isLoading.value) return;

        if (!passphrase.value) {
            enterError.value = 'Passphrase can\'t be empty';

            return;
        }

        isLoading.value = true;

        try {
            await setAccess();
            store.dispatch(OBJECTS_ACTIONS.SET_PASSPHRASE, passphrase.value);

            activeStep.value = CreateProjectPassphraseStep.Success;
        } catch (e) {
            await notify.error(e.message);
        } finally {
            isLoading.value = false;
        }

        return;
    }

    if (activeStep.value === CreateProjectPassphraseStep.Success) {
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
        if (passphraseSaved.value) {
            passphraseSaved.value = false;
        }

        activeStep.value = CreateProjectPassphraseStep.SelectMode;
        return;
    }
}
</script>

<style scoped lang="scss">
.modal {
    padding: 43px 60px 53px;
    font-family: 'font_regular', sans-serif;

    @media screen and (max-width: 615px) {
        padding: 30px 20px;
    }

    &__buttons {
        display: flex;
        align-items: center;
        justify-content: center;
        column-gap: 33px;
        margin-top: 20px;

        @media screen and (max-width: 530px) {
            column-gap: unset;
            flex-direction: column-reverse;
            row-gap: 15px;
        }
    }

    &__save-container {
        padding: 14px 20px;
        display: flex;
        align-items: center;
        cursor: pointer;
        margin-top: 16px;
        background: #fafafb;
        border: 1px solid #c8d3de;
        border-radius: 10px;

        &__check {
            background: #fff;
            border: 1px solid #c8d3de;
            border-radius: 8px;
            min-width: 32px;
            min-height: 32px;
            display: flex;
            align-items: center;
            justify-content: center;
        }

        &__info {
            margin-left: 12px;

            &__title {
                font-family: 'font_bold', sans-serif;
                font-size: 14px;
                line-height: 20px;
                color: #091c45;
                margin-bottom: 8px;
                text-align: left;
            }

            &__msg {
                font-size: 12px;
                line-height: 18px;
                color: #091c45;
                text-align: left;
            }
        }
    }
}

.checked {
    background: #00ac26;
    border-color: #00ac26;
}
</style>
