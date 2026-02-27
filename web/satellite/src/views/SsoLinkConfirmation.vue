// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-container class="fill-height" fluid>
        <v-row justify="center" align="center">
            <v-col cols="12" sm="9" md="7" lg="5" xl="4" xxl="3">
                <v-card class="pa-2 pa-sm-7">
                    <h2 class="mb-3">Verify your email to finish linking</h2>
                    <p>We sent a 6 digit verification code to your email address.</p>
                    <v-alert
                        v-if="isUnauthorizedMessageShown"
                        variant="tonal"
                        color="error"
                        title="Invalid Code"
                        text="Verification failed. Please check the code and try again."
                        density="comfortable"
                        class="mt-4 mb-3"
                        border
                    />
                    <v-form class="mt-4" @submit.prevent="verifyCode">
                        <v-card class="my-4" rounded="lg" color="secondary" variant="outlined">
                            <v-otp-input
                                :model-value="code"
                                :error="isError"
                                :disabled="isLoading"
                                autofocus
                                class="my-2"
                                @update:model-value="onValueChange"
                            />
                        </v-card>

                        <v-btn
                            type="submit"
                            :disabled="code.length < 6"
                            :loading="isLoading"
                            color="primary"
                            size="large"
                            block
                        >
                            Verify and Continue
                        </v-btn>
                    </v-form>
                    <p class="text-body-2 mt-4 text-center">
                        Need a new code? Restart the sign-in flow from
                        <router-link class="link font-weight-bold" :to="ROUTES.Login.path">login</router-link>.
                    </p>
                </v-card>
            </v-col>
        </v-row>
    </v-container>
</template>

<script setup lang="ts">
import { ref } from 'vue';
import { useRouter } from 'vue-router';
import {
    VBtn,
    VCard,
    VCol,
    VContainer,
    VForm,
    VRow,
    VOtpInput,
    VAlert,
} from 'vuetify/components';

import { AuthHttpApi } from '@/api/auth';
import { ErrorUnauthorized } from '@/api/errors/ErrorUnauthorized';
import { useLoading } from '@/composables/useLoading';
import { useNotify } from '@/composables/useNotify';
import { useUsersStore } from '@/store/modules/usersStore';
import { useAppStore } from '@/store/modules/appStore';
import { LocalData } from '@/utils/localData';
import { ROUTES } from '@/router';

const auth: AuthHttpApi = new AuthHttpApi();
const router = useRouter();
const notify = useNotify();
const usersStore = useUsersStore();
const appStore = useAppStore();
const { isLoading, withLoading } = useLoading();

const code = ref('');
const isError = ref(false);
const isUnauthorizedMessageShown = ref(false);

function onValueChange(value: string) {
    const val = value.slice(0, 6);
    if (isNaN(+val)) {
        return;
    }
    code.value = val;
}

function verifyCode(): void {
    isError.value = false;
    isUnauthorizedMessageShown.value = false;

    if (code.value.length !== 6) {
        isError.value = true;
        return;
    }

    withLoading(async () => {
        try {
            const tokenInfo = await auth.verifySsoLink(code.value);
            LocalData.setSessionExpirationDate(tokenInfo.expiresAt);
            LocalData.removeSessionHasExpired();
        } catch (error) {
            if (error instanceof ErrorUnauthorized) {
                isUnauthorizedMessageShown.value = true;
                return;
            }
            notify.notifyError(error);
            isError.value = true;
            return;
        }

        appStore.toggleHasJustLoggedIn(true);
        usersStore.login();
        await router.push(ROUTES.Projects.path);
    });
}
</script>
