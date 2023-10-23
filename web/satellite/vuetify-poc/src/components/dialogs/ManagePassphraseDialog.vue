// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-dialog v-model="model" max-width="420" transition="fade-transition">
        <v-card ref="innerContent" rounded="xlg">
            <v-card-item class="pl-7 py-4">
                <template #prepend>
                    <img class="d-block" :src="stepInfo[step].ref.value?.iconSrc || LockIcon">
                </template>

                <v-card-title class="font-weight-bold">
                    {{ stepInfo[step].ref.value?.title }}
                </v-card-title>

                <template #append>
                    <v-btn icon="$close" variant="text" size="small" color="default" @click="model = false" />
                </template>
            </v-card-item>

            <v-divider />

            <v-window v-model="step" class="overflow-y-auto">
                <v-window-item :value="ManageProjectPassphraseStep.ManageOptions">
                    <manage-options-step
                        :ref="stepInfo[ManageProjectPassphraseStep.ManageOptions].ref"
                        @option-click="newStep => step = newStep"
                    />
                </v-window-item>

                <v-window-item :value="ManageProjectPassphraseStep.Create">
                    <create-step :ref="stepInfo[ManageProjectPassphraseStep.Create].ref" />
                </v-window-item>

                <v-window-item :value="ManageProjectPassphraseStep.EncryptionPassphrase">
                    <encryption-passphrase-step
                        :ref="stepInfo[ManageProjectPassphraseStep.EncryptionPassphrase].ref"
                        @select-option="newOpt => passphraseOption = newOpt"
                    />
                </v-window-item>

                <v-window-item :value="ManageProjectPassphraseStep.EnterPassphrase">
                    <enter-passphrase-step
                        :ref="stepInfo[ManageProjectPassphraseStep.EnterPassphrase].ref"
                        ack-required
                        @passphrase-changed="newPass => passphrase = newPass"
                    >
                        Please note that Storj does not know or store your encryption passphrase.
                        If you lose it, you will not be able to recover your files.
                    </enter-passphrase-step>
                </v-window-item>

                <v-window-item :value="ManageProjectPassphraseStep.PassphraseGenerated">
                    <passphrase-generated-step
                        :ref="stepInfo[ManageProjectPassphraseStep.PassphraseGenerated].ref"
                        :name="projectName"
                        @passphrase-changed="newPass => passphrase = newPass"
                    >
                        Please note that Storj does not know or store your encryption passphrase.
                        If you lose it, you will not be able to recover your files.
                    </passphrase-generated-step>
                </v-window-item>

                <v-window-item :value="ManageProjectPassphraseStep.Success">
                    <success-step
                        :ref="stepInfo[ManageProjectPassphraseStep.Success].ref"
                        :passphrase="passphrase"
                        :option="passphraseOption"
                    />
                </v-window-item>

                <v-window-item :value="ManageProjectPassphraseStep.Switch">
                    <enter-passphrase-step
                        :ref="stepInfo[ManageProjectPassphraseStep.Switch].ref"
                        title="Switch Passphrase"
                        set-on-next
                    >
                        Switch passphrase to view existing data that is uploaded with a different passphrase, or upload new data.
                        Please note that you won't see the previous data once you switch passphrases.
                    </enter-passphrase-step>
                </v-window-item>

                <v-window-item :value="ManageProjectPassphraseStep.Clear">
                    <clear-step :ref="stepInfo[ManageProjectPassphraseStep.Clear].ref" />
                </v-window-item>

                <!-- This is required to prevent the above item from sliding in the wrong direction when Back is clicked. -->
                <v-window-item />
            </v-window>

            <v-divider />

            <v-card-actions class="pa-7">
                <v-row>
                    <v-col v-if="stepInfo[step].prev.value">
                        <v-btn
                            variant="outlined"
                            color="default"
                            prepend-icon="mdi-chevron-left"
                            block
                            @click="onBackClick"
                        >
                            Back
                        </v-btn>
                    </v-col>
                    <v-col v-else-if="stepInfo[step].showCancelButton">
                        <v-btn variant="outlined" color="default" block @click="model = false">
                            Cancel
                        </v-btn>
                    </v-col>
                    <v-col v-if="stepInfo[step].next.value || stepInfo[step].showNextButton">
                        <v-btn
                            color="primary"
                            variant="flat"
                            block
                            append-icon="mdi-chevron-right"
                            @click="onNextClick"
                        >
                            Continue
                        </v-btn>
                    </v-col>
                </v-row>
            </v-card-actions>
        </v-card>
    </v-dialog>
</template>

<script setup lang="ts">
import { Component, Ref, WatchStopHandle, computed, ref, watch } from 'vue';
import {
    VDialog,
    VCard,
    VCardItem,
    VCardTitle,
    VWindow,
    VWindowItem,
    VDivider,
    VCardActions,
    VRow,
    VCol,
    VBtn,
} from 'vuetify/components';

import { ManageProjectPassphraseStep, PassphraseOption } from '@poc/types/managePassphrase';
import { useProjectsStore } from '@/store/modules/projectsStore';
import { DialogStepComponent } from '@poc/types/common';

import ManageOptionsStep from '@poc/components/dialogs/managePassphraseSteps/ManageOptionsStep.vue';
import CreateStep from '@poc/components/dialogs/managePassphraseSteps/CreateStep.vue';
import EncryptionPassphraseStep from '@poc/components/dialogs/managePassphraseSteps/EncryptionPassphraseStep.vue';
import EnterPassphraseStep from '@poc/components/dialogs/commonPassphraseSteps/EnterPassphraseStep.vue';
import PassphraseGeneratedStep from '@poc/components/dialogs/commonPassphraseSteps/PassphraseGeneratedStep.vue';
import SuccessStep from '@poc/components/dialogs/managePassphraseSteps/SuccessStep.vue';
import ClearStep from '@poc/components/dialogs/managePassphraseSteps/ClearStep.vue';

import LockIcon from '@/../static/images/accessGrants/newCreateFlow/accessEncryption.svg';

type ManagePassphraseLocation = ManageProjectPassphraseStep | null | (() => (ManageProjectPassphraseStep | null));

class StepInfo {
    public ref: Ref<DialogStepComponent | null> = ref<DialogStepComponent | null>(null);
    public prev: Ref<ManageProjectPassphraseStep | null>;
    public next: Ref<ManageProjectPassphraseStep | null>;

    constructor(
        prev: ManagePassphraseLocation = null,
        next: ManagePassphraseLocation = null,
        public showCancelButton: boolean = true,
        public showNextButton: boolean = true,
    ) {
        this.prev = (typeof prev === 'function')
            ? computed<ManageProjectPassphraseStep | null>(prev)
            : ref<ManageProjectPassphraseStep | null>(prev);
        this.next = (typeof next === 'function')
            ? computed<ManageProjectPassphraseStep | null>(next)
            : ref<ManageProjectPassphraseStep | null>(next);
    }
}

const props = withDefaults(defineProps<{
    modelValue: boolean;
    isCreate: boolean;
}>(), {
    modelValue: false,
    isCreate: false,
});

const model = computed<boolean>({
    get: () => props.modelValue,
    set: value => emit('update:modelValue', value),
});

const emit = defineEmits<{
    'update:modelValue': [value: boolean];
}>();

const projectsStore = useProjectsStore();

const innerContent = ref<Component | null>(null);
const step = ref<ManageProjectPassphraseStep>(
    props.isCreate
        ? ManageProjectPassphraseStep.EncryptionPassphrase
        : ManageProjectPassphraseStep.ManageOptions,
);
const passphraseOption = ref<PassphraseOption>(PassphraseOption.EnterPassphrase);
const passphrase = ref<string>('');

const projectName = computed<string>(() => projectsStore.state.selectedProject.name);

const stepInfo: Record<ManageProjectPassphraseStep, StepInfo> = {
    [ManageProjectPassphraseStep.ManageOptions]: new StepInfo(null, null, true, false),

    [ManageProjectPassphraseStep.Create]: new StepInfo(
        ManageProjectPassphraseStep.ManageOptions,
        ManageProjectPassphraseStep.EncryptionPassphrase,
    ),
    [ManageProjectPassphraseStep.EncryptionPassphrase]: new StepInfo(
        () => props.isCreate ? null : ManageProjectPassphraseStep.Create,
        () => passphraseOption.value === PassphraseOption.GeneratePassphrase
            ? ManageProjectPassphraseStep.PassphraseGenerated
            : ManageProjectPassphraseStep.EnterPassphrase,
    ),
    [ManageProjectPassphraseStep.PassphraseGenerated]: new StepInfo(
        ManageProjectPassphraseStep.EncryptionPassphrase,
        ManageProjectPassphraseStep.Success,
    ),
    [ManageProjectPassphraseStep.EnterPassphrase]: new StepInfo(
        ManageProjectPassphraseStep.EncryptionPassphrase,
        ManageProjectPassphraseStep.Success,
    ),
    [ManageProjectPassphraseStep.Success]: new StepInfo(null, null, false),

    [ManageProjectPassphraseStep.Switch]: new StepInfo(ManageProjectPassphraseStep.ManageOptions),
    [ManageProjectPassphraseStep.Clear]: new StepInfo(ManageProjectPassphraseStep.ManageOptions),
};

function onBackClick(): void {
    const info = stepInfo[step.value];

    info.ref.value?.onExit?.('prev');

    const prev = info.prev.value;
    if (prev !== null) {
        step.value = prev;
        return;
    }

    model.value = false;
}

function onNextClick(): void {
    const info = stepInfo[step.value];

    if (info.ref.value?.validate?.() === false) return;

    info.ref.value?.onExit?.('next');

    const next = info.next.value;
    if (next !== null) {
        step.value = next;
        return;
    }

    model.value = false;
}

/**
 * Initializes a step when it has been entered.
 */
watch(step, newStep => {
    if (!innerContent.value) return;

    // Window items are lazy loaded, so the component may not exist yet
    let unwatch: WatchStopHandle | null = null;
    let unwatchImmediately = false;
    unwatch = watch(
        () => stepInfo[newStep].ref.value,
        stepComp => {
            if (!stepComp) return;
            stepComp.onEnter?.();
            if (unwatch) {
                unwatch();
                return;
            }
            unwatchImmediately = true;
        },
        { immediate: true },
    );
    if (unwatchImmediately) unwatch();
});

watch(innerContent, comp => {
    if (comp) return;
    step.value = props.isCreate
        ? ManageProjectPassphraseStep.EncryptionPassphrase
        : ManageProjectPassphraseStep.ManageOptions;
});
</script>
