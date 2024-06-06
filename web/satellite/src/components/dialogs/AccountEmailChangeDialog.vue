// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-dialog
        v-model="model"
        activator="parent"
        width="auto"
        min-width="400px"
        max-width="450px"
        transition="fade-transition"
        persistent
    >
        <v-card rounded="xlg">
            <v-sheet>
                <v-card-item class="pa-6">
                    <template #prepend>
                        <v-sheet
                            class="border-sm d-flex justify-center align-center"
                            width="40"
                            height="40"
                            rounded="lg"
                        >
                            <IconEmail />
                        </v-sheet>
                    </template>
                    <v-card-title class="font-weight-bold">
                        Change Email
                    </v-card-title>
                    <template #append>
                        <v-btn
                            icon="$close"
                            variant="text"
                            size="small"
                            color="default"
                            @click="model = false"
                        />
                    </template>
                </v-card-item>
            </v-sheet>

            <v-divider />

            <v-window v-model="step">
                <v-window-item :value="ChangeEmailStep.InitStep">
                    <v-form class="pa-6" @submit.prevent="proceed">
                        <v-row>
                            <v-col>
                                <p class="mb-4">You are about to change your email address associated with your Storj account.</p>
                                <p>Account:</p>
                                <v-chip
                                    variant="tonal"
                                    class="font-weight-bold"
                                >
                                    {{ user.email }}
                                </v-chip>
                            </v-col>
                        </v-row>
                    </v-form>
                </v-window-item>

                <v-window-item :value="ChangeEmailStep.VerifyPasswordStep">
                    <v-form v-model="passwordFormValid" class="pa-6" @submit.prevent="proceed">
                        <v-row>
                            <v-col>
                                <p>Enter your account password to continue.</p>
                                <v-text-field
                                    type="password"
                                    label="Password"
                                    class="mt-6"
                                    :rules="[RequiredRule]"
                                    required
                                />
                            </v-col>
                        </v-row>
                    </v-form>
                </v-window-item>

                <v-window-item :value="ChangeEmailStep.Verify2faStep">
                    <v-form v-model="mfaFormValid" class="pa-6" @submit.prevent="proceed">
                        <v-row>
                            <v-col>
                                <p>Enter the code from your 2FA application.</p>
                                <v-otp-input
                                    class="mt-6"
                                    type="number"
                                    autofocus
                                    maxlength="6"
                                />
                            </v-col>
                        </v-row>
                    </v-form>
                </v-window-item>

                <v-window-item :value="ChangeEmailStep.VerifyOldEmailStep">
                    <v-form v-model="verifyOldFormValid" class="pa-6" @submit.prevent="proceed">
                        <v-row>
                            <v-col>
                                <p>Enter the code you received on your old email.</p>
                                <v-otp-input
                                    class="mt-6"
                                    type="number"
                                    autofocus
                                    maxlength="6"
                                />
                            </v-col>
                        </v-row>
                    </v-form>
                </v-window-item>

                <v-window-item :value="ChangeEmailStep.SetNewEmailStep">
                    <v-form v-model="newEmailFormValid" class="pa-6" @submit.prevent="proceed">
                        <v-row>
                            <v-col>
                                <p>Enter your new email address.</p>
                                <v-text-field
                                    type="email"
                                    label="Email"
                                    class="mt-6"
                                    :rules="[RequiredRule, EmailRule]"
                                    required
                                />
                            </v-col>
                        </v-row>
                    </v-form>
                </v-window-item>

                <v-window-item :value="ChangeEmailStep.VerifyNewEmailStep">
                    <v-form v-model="verifyNewFormValid" class="pa-6" @submit.prevent="proceed">
                        <v-row>
                            <v-col>
                                <p>Enter the code you received on your new email.</p>
                                <v-otp-input
                                    class="mt-6"
                                    type="number"
                                    autofocus
                                    maxlength="6"
                                />
                            </v-col>
                        </v-row>
                    </v-form>
                </v-window-item>

                <v-window-item :value="ChangeEmailStep.SuccessStep">
                    <v-form class="pa-6" @submit.prevent="proceed">
                        <v-row>
                            <v-col>
                                <p>Your email has been successfully updated.</p>
                            </v-col>
                        </v-row>
                    </v-form>
                </v-window-item>
            </v-window>

            <v-divider />

            <v-card-actions class="pa-6">
                <v-row>
                    <v-col>
                        <v-btn
                            v-if="step === ChangeEmailStep.InitStep || step === ChangeEmailStep.SuccessStep"
                            variant="outlined"
                            color="default"
                            block
                            @click="model = false"
                        >
                            {{ step === ChangeEmailStep.InitStep ? 'Cancel' : 'Close' }}
                        </v-btn>

                        <v-btn
                            v-if="step > ChangeEmailStep.InitStep && step < ChangeEmailStep.SuccessStep"
                            variant="outlined"
                            color="default"
                            block
                            @click="step--"
                        >
                            Back
                        </v-btn>
                    </v-col>

                    <v-col>
                        <v-btn
                            v-if="step < ChangeEmailStep.SuccessStep"
                            color="primary"
                            variant="flat"
                            block
                            @click="proceed"
                        >
                            Next
                        </v-btn>

                        <v-btn
                            v-if="step === ChangeEmailStep.SuccessStep"
                            color="primary"
                            variant="flat"
                            block
                            @click="model = false"
                        >
                            Finish
                        </v-btn>
                    </v-col>
                </v-row>
            </v-card-actions>
        </v-card>
    </v-dialog>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue';
import {
    VBtn,
    VCard,
    VCardActions,
    VCardItem,
    VCardTitle,
    VChip,
    VCol,
    VDialog,
    VDivider,
    VForm,
    VOtpInput,
    VRow,
    VSheet,
    VTextField,
    VWindow,
    VWindowItem,
} from 'vuetify/components';

import { ChangeEmailStep } from '@/types/changeEmail';
import { useUsersStore } from '@/store/modules/usersStore';
import { User } from '@/types/users';
import { EmailRule, RequiredRule } from '@/types/common';

import IconEmail from '@/components/icons/IconEmail.vue';

const userStore = useUsersStore();

const model = defineModel<boolean>({ required: true });

const step = ref<ChangeEmailStep>(ChangeEmailStep.InitStep);
const passwordFormValid = ref<boolean>(false);
const mfaFormValid = ref<boolean>(false);
const verifyOldFormValid = ref<boolean>(false);
const verifyNewFormValid = ref<boolean>(false);
const newEmailFormValid = ref<boolean>(false);

const user = computed<User>(() => userStore.state.user);

async function proceed(): Promise<void> {
    switch (step.value) {
    case ChangeEmailStep.InitStep:
        step.value = ChangeEmailStep.VerifyPasswordStep;
        break;
    case ChangeEmailStep.VerifyPasswordStep:
        //TODO
        break;
    case ChangeEmailStep.Verify2faStep:
        //TODO
        break;
    case ChangeEmailStep.VerifyOldEmailStep:
        //TODO
        break;
    case ChangeEmailStep.SetNewEmailStep:
        //TODO
        break;
    case ChangeEmailStep.VerifyNewEmailStep:
        //TODO
        break;
    case ChangeEmailStep.SuccessStep:
        //TODO
    }
}

watch(model, val => {
    if (!val) step.value = ChangeEmailStep.InitStep;
});
</script>
