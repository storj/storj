// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-dialog v-model="model" transition="fade-transition" max-width="700">
        <v-card
            rounded="xlg"
            :title="title"
            :subtitle="step === Step.InputAccess ? 'Enter an access to inspect' : ''"
        >
            <template #append>
                <v-btn
                    :icon="X" :disabled="isLoading"
                    variant="text" size="small" color="default" @click="model = false"
                />
            </template>

            <v-window v-model="step" :touch="false">
                <div class="pa-6">
                    <v-window-item :value="Step.InputAccess">
                        <v-row>
                            <v-col cols="12">
                                <v-textarea
                                    v-model="access"
                                    label="Access"
                                    variant="solo-filled"
                                    :rules="[RequiredRule]"
                                    placeholder="Enter access"
                                    hide-details="auto"
                                    flat
                                    autofocus
                                    required
                                    auto-grow
                                />
                            </v-col>
                        </v-row>
                    </v-window-item>
                    <v-window-item :value="Step.Result">
                        <v-row>
                            <v-col cols="12">
                                <v-alert
                                    v-if="result?.revoked"
                                    type="warning"
                                    variant="tonal"
                                    density="compact"
                                    class="mb-4"
                                >
                                    This access is revoked.
                                </v-alert>
                                <p class="mb-3"><b>Public Project ID: </b><br>{{ result?.publicProjectID }}</p>
                                <p class="mb-3"><b>Project Owner Email: </b><br>{{ result?.projectOwnerEmail }}</p>
                                <p class="mb-3"><b>Project Owner ID: </b><br>{{ result?.projectOwnerID }}</p>
                                <p class="mb-3"><b>Creator ID: </b><br>{{ result?.creatorID || '' }}</p>
                                <p class="mb-3"><b>API Key: </b><br>{{ result?.apiKey }}</p>
                                <p class="mb-3"><b>Satellite Address: </b><br>{{ result?.satelliteAddr }}</p>
                                <p class="mb-3"><b>Default Path Cipher: </b><br>{{ result?.defaultPathCipher }}</p>
                                <template v-if="result && result.macaroon.caveats?.length">
                                    <div v-for="(caveat, index) in result.macaroon.caveats" :key="index">
                                        <p><b>Caveat {{ index + 1 }}:</b></p>
                                        <ul class="ml-4">
                                            <template v-for="{ key, value } in formatCaveat(caveat)" :key="key">
                                                <li v-if="Array.isArray(value)">
                                                    <b>{{ key }}:</b>
                                                    <ul class="ml-4">
                                                        <li v-for="(path, pi) in value" :key="pi">
                                                            {{ path }}
                                                        </li>
                                                    </ul>
                                                </li>
                                                <li v-else>
                                                    <b>{{ key }}:</b> {{ value }}
                                                </li>
                                            </template>
                                        </ul>
                                    </div>
                                </template>
                            </v-col>
                        </v-row>
                    </v-window-item>
                    <v-window-item :value="Step.ConfirmRevoke">
                        <v-row>
                            <v-col cols="12">
                                <p>Are you sure you want to revoke this access?</p>
                                <p><b>Note: </b>This action may take some time to apply.</p>
                            </v-col>
                            <v-col cols="12">
                                <v-textarea
                                    v-model="reason"
                                    :rules="[RequiredRule]"
                                    placeholder="Enter reason for revoking this Access"
                                    label="Reason"
                                    variant="solo-filled"
                                    hide-details="auto"
                                    autofocus
                                    flat
                                />
                            </v-col>
                        </v-row>
                    </v-window-item>
                </div>
            </v-window>

            <v-card-actions class="pa-6">
                <v-row>
                    <v-col v-if="step !== Step.ConfirmRevoke">
                        <v-btn variant="outlined" color="default" block @click="model = false">Cancel</v-btn>
                    </v-col>
                    <v-col v-if="step === Step.InputAccess">
                        <v-btn
                            variant="flat"
                            :loading="isLoading"
                            :disabled="!access"
                            block
                            @click="inspect"
                        >
                            Inspect
                        </v-btn>
                    </v-col>
                    <v-col v-if="step === Step.Result && isRevocable">
                        <v-btn
                            variant="flat"
                            color="error"
                            :loading="isLoading"
                            block
                            @click="step = Step.ConfirmRevoke"
                        >
                            Revoke
                        </v-btn>
                    </v-col>
                    <template v-if="step === Step.ConfirmRevoke">
                        <v-col>
                            <v-btn
                                variant="outlined"
                                color="default"
                                block
                                @click="step = Step.Result"
                            >
                                No
                            </v-btn>
                        </v-col>
                        <v-col>
                            <v-btn
                                variant="flat"
                                color="error"
                                :loading="isLoading"
                                :disabled="!reason"
                                block
                                @click="revoke"
                            >
                                Yes
                            </v-btn>
                        </v-col>
                    </template>
                </v-row>
            </v-card-actions>
        </v-card>
    </v-dialog>
</template>

<script setup lang="ts">
import {
    VBtn,
    VCard,
    VCardActions,
    VCol,
    VDialog,
    VRow,
    VTextarea,
    VWindow,
    VWindowItem,
    VAlert,
} from 'vuetify/components';
import { X } from 'lucide-vue-next';
import { computed, ref, watch } from 'vue';

import { useLoading } from '@/composables/useLoading';
import { useNotify } from '@/composables/useNotify';
import { RequiredRule } from '@/types/common';
import { useAccessStore } from '@/store/access';
import { AccessInspectResult, Caveat, Caveat_Path } from '@/api/client.gen';
import { useAppStore } from '@/store/app';

enum Step {
    InputAccess,
    Result,
    ConfirmRevoke,
}

const accessStore = useAccessStore();
const appStore = useAppStore();

const notify = useNotify();
const { isLoading, withLoading } = useLoading();

const model = defineModel<boolean>({ required: true });

const step = ref<Step>(Step.InputAccess);
const access = ref<string>('');
const result = ref<AccessInspectResult>();
const reason = ref<string>('');

const isRevocable = computed<boolean>(() => {
    if (!appStore.state.settings.admin.features.access.revoke) return false;

    const val = result.value;

    return val !== undefined && !val.revoked && val.macaroon.tail !== null;
});

const title = computed<string>(() => {
    switch (step.value) {
    case Step.InputAccess:
        return 'Inspect Access';
    case Step.Result:
        return 'Access Metadata';
    case Step.ConfirmRevoke:
        return 'Confirm Revoke';
    default:
        return '';
    }
});

function inspect(): void {
    withLoading(async () => {
        try {
            result.value = await accessStore.inspectAccess(access.value);
            step.value = Step.Result;
        } catch (e) {
            notify.error(`Failed to inspect access. ${e.message}`);
        }
    });
}

function revoke(): void {
    withLoading(async () => {
        if (!isRevocable.value) return;

        try {
            await accessStore.revokeAccess(result.value?.macaroon.tail ?? '', result.value?.apiKeyID ?? '', reason.value);
            notify.success('Access revoke initiated successfully.');
            model.value = false;
        } catch (e) {
            notify.error(`Failed to revoke access. ${e.message}`);
        }
    });
}

type CaveatField = { key: string; value: string | string[] };

function formatCaveat(caveat: Caveat): CaveatField[] {
    const result: CaveatField[] = [];

    for (const [key, val] of Object.entries(caveat)) {
        if (val === null || val === undefined) continue;

        if (key === 'allowed_paths' && Array.isArray(val)) {
            const paths = (val as Caveat_Path[]).map(p => {
                const parts: string[] = [];
                if (p.bucket) parts.push(`bucket: ${p.bucket}`);
                if (p.encrypted_path_prefix) parts.push(`prefix: ${p.encrypted_path_prefix}`);
                return parts.join(', ') || '(empty path)';
            });

            result.push({ key, value: paths });
        } else {
            result.push({ key, value: String(val) });
        }
    }
    return result;
}

watch(model, (newVal) => {
    if (!newVal) {
        setTimeout(() => {
            step.value = Step.InputAccess;
            access.value = '';
            result.value = undefined;
            reason.value = '';
        }, 500);
    }
});
</script>
