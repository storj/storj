// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-container>
        <v-row>
            <v-col>
                <v-card v-if="state === State.Success" max-width="800" class="mx-auto pa-8 pt-5">
                    <v-card-item class="pl-0">
                        <v-icon color="success" size="42" class="mb-5">
                            <template #default>
                                <component :is="CircleCheck" :size="42" />
                            </template>
                        </v-icon>
                        <v-card-title class="text-h4 pb-4">
                            Application Submitted Successfully!
                        </v-card-title>
                        <v-card-text class="pl-0">
                            <p class="text-body-1">
                                Thank you for applying to join the cunoFS Beta program.
                                We've received your application and are excited about your interest
                                in helping shape the future of high-performance storage.
                            </p>
                        </v-card-text>
                        <v-card class="pa-5 mt-5 mb-10">
                            <h3 class="mb-2 font-weight-medium">
                                What happens next?
                            </h3>
                            <p class="my-4">
                                Our team will review your application as soon as possible.
                            </p>
                            <p class="my-4">
                                You'll receive an email with your license and setup instructions.
                            </p>
                            <p class="my-4">
                                You'll get exclusive access to cunoFS for a 14-day trial period.
                            </p>
                            <p class="mt-4">
                                Our support team will be available to help you get started.
                            </p>
                        </v-card>
                        <v-btn
                            color="secondary"
                            link
                            href="https://cuno-cunofs.readthedocs-hosted.com/en/stable/getting-started-download-and-installation.html"
                            target="_blank"
                            rel="noopener noreferrer"
                        >
                            CunoFS Install Guide
                        </v-btn>
                    </v-card-item>
                </v-card>
                <v-card v-else class="pa-4 pa-sm-8 mx-auto" max-width="800">
                    <img src="@/assets/storj-plus-cuno.webp" alt="Storj + cunoFS" class="w-100 rounded-lg mb-4">

                    <h1 class="mb-2">
                        Join the Insider Beta for cunoFS
                    </h1>

                    <p class="text-subtitle-1 mb-5">
                        Be among the first to try cunoFS with 14-day free trial access and help us shape the future of high-performance storage. Available for Linux, macOS, and Windows.
                    </p>

                    <v-row>
                        <v-col>
                            <v-card class="pa-4 h-100" variant="outlined">
                                <v-icon class="mb-3"><svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="lucide lucide-folder-check"><path d="M20 20a2 2 0 0 0 2-2V8a2 2 0 0 0-2-2h-7.9a2 2 0 0 1-1.69-.9L9.6 3.9A2 2 0 0 0 7.93 3H4a2 2 0 0 0-2 2v13a2 2 0 0 0 2 2Z" /><path d="m9 13 2 2 4-4" /></svg></v-icon>
                                <p class="text-subtitle-2">
                                    Mount object storage as a local drive with real-time access.
                                </p>
                            </v-card>
                        </v-col>
                        <v-col>
                            <v-card class="pa-4 h-100" variant="outlined">
                                <v-icon class="mb-3">
                                    <svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="lucide lucide-database-zap"><ellipse cx="12" cy="5" rx="9" ry="3" /><path d="M3 5V19A9 3 0 0 0 15 21.84" /><path d="M21 5V8" /><path d="M21 12L18 17H22L19 22" /><path d="M3 12A9 3 0 0 0 14.59 14.87" /></svg>
                                </v-icon>
                                <p class="text-subtitle-2">
                                    High-throughput access to large files without download delays
                                </p>
                            </v-card>
                        </v-col>
                        <v-col>
                            <v-card class="pa-4 h-100" variant="outlined">
                                <v-icon class="mb-3">
                                    <svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="lucide lucide-cloud"><path d="M17.5 19H9a7 7 0 1 1 6.71-9h1.79a4.5 4.5 0 1 1 0 9Z" /></svg>
                                </v-icon>
                                <p class="text-subtitle-2">
                                    Works with your Storj buckets and any other object storage.
                                </p>
                            </v-card>
                        </v-col>
                    </v-row>

                    <h3 class="mt-7">
                        Apply for Beta Access
                    </h3>

                    <p class="text-subtitle-1 mt-2 mb-9">
                        Get exclusive early access to cunoFS and experience high-speed, local file system mounting for your buckets. Help us refine the product with your feedback.
                    </p>

                    <v-form v-model="formValid" @submit.prevent="submitForm">
                        <v-text-field
                            :model-value="userEmail"
                            label="Email"
                            readonly
                            variant="outlined"
                            class="mb-4"
                        />

                        <v-text-field
                            v-model="formData.organization"
                            label="Organization"
                            placeholder="Enter your organization name"
                            variant="outlined"
                            required
                            :rules="[RequiredRule, MaxNameLengthRule]"
                            class="mb-4"
                        />

                        <v-select
                            v-model="formData.industry"
                            :items="industries"
                            label="Industry/Use Case"
                            variant="outlined"
                            required
                            :rules="[RequiredRule, MaxNameLengthRule]"
                            class="mb-4"
                        />
                        <v-text-field
                            v-if="formData.industry === OtherLabel"
                            v-model="otherIndustry"
                            label="Other Industry/Use Case"
                            variant="outlined"
                            required
                            :rules="[RequiredRule, MaxNameLengthRule]"
                            class="mb-4"
                        />

                        <v-select
                            v-model="formData.operatingSystem"
                            :items="operatingSystems"
                            label="Operating System"
                            required
                            :rules="[RequiredRule]"
                            class="mb-4"
                        />

                        <v-select
                            v-model="formData.teamSize"
                            :items="teamSizes"
                            label="Team Size"
                            required
                            :rules="[RequiredRule]"
                            class="mb-4"
                        />

                        <v-select
                            v-model="formData.storageUsage"
                            :items="storageRanges"
                            label="Current Storage Usage"
                            required
                            :rules="[RequiredRule]"
                            class="mb-4"
                        />

                        <v-card-text class="px-0">
                            <div class="text-subtitle-1 mb-2 font-weight-bold">Infrastructure Type</div>
                            <v-checkbox
                                v-for="type in infrastructureTypes"
                                :key="type"
                                v-model="formData.infrastructureType"
                                :label="type"
                                :value="type"
                                density="compact"
                                hide-details
                            />
                        </v-card-text>

                        <v-card-text class="px-0">
                            <div class="text-subtitle-1 mb-2 font-weight-bold">Current Storage Backends</div>
                            <v-checkbox
                                v-for="backend in storageBackends"
                                :key="backend"
                                v-model="formData.storageBackend"
                                :label="backend"
                                :value="backend"
                                density="compact"
                                hide-details
                            />
                            <v-text-field
                                v-if="formData.storageBackend.includes(OtherLabel)"
                                v-model="otherStorageBackend"
                                label="Other Storage Backend"
                                variant="outlined"
                                class="mt-4"
                                :rules="[MaxNameLengthRule]"
                                hide-details
                            />
                        </v-card-text>

                        <v-card-text class="px-0">
                            <div class="text-subtitle-1 mb-2 font-weight-bold">Current Storage Mount Solution</div>
                            <v-checkbox
                                v-for="solution in mountSolutions"
                                :key="solution"
                                v-model="formData.mountSolution"
                                :label="solution"
                                :value="solution"
                                density="compact"
                                hide-details
                            />
                            <v-text-field
                                v-if="formData.mountSolution.includes(OtherLabel)"
                                v-model="otherMountSolution"
                                label="Other Mount Solution"
                                variant="outlined"
                                class="mt-4"
                                :rules="[MaxNameLengthRule]"
                                hide-details
                            />
                        </v-card-text>

                        <v-card-text class="px-0">
                            <div class="text-subtitle-1 mb-2 font-weight-bold">Desired Features</div>
                            <v-checkbox
                                v-for="feature in desiredFeatures"
                                :key="feature"
                                v-model="formData.desiredFeatures"
                                :label="feature"
                                :value="feature"
                                density="compact"
                                hide-details
                            />
                        </v-card-text>

                        <v-card-text class="px-0">
                            <div class="text-subtitle-1 mb-2 font-weight-bold">Current Pain Points</div>
                            <v-checkbox
                                v-for="point in painPoints"
                                :key="point"
                                v-model="formData.painPoints"
                                :label="point"
                                :value="point"
                                density="compact"
                                hide-details
                            />
                        </v-card-text>

                        <v-textarea
                            v-model="formData.comments"
                            label="What specific tasks or workflows will you use cunoFS for?"
                            placeholder="What specific tasks or workflows will you use cunoFS for?"
                            variant="outlined"
                            class="rounded-lg mt-4"
                            maxlength="500"
                            rows="3"
                        />

                        <v-btn
                            type="submit"
                            color="primary"
                            block
                            :loading="isLoading"
                            :disabled="!formValid"
                            class="mt-4"
                        >
                            Submit Application
                        </v-btn>
                    </v-form>
                </v-card>
            </v-col>
        </v-row>
    </v-container>
</template>

<script setup lang="ts">
import { ref, reactive, onBeforeMount, computed, watch } from 'vue';
import {
    VContainer,
    VRow,
    VCol,
    VCard,
    VCardItem,
    VCardTitle,
    VCardText,
    VCheckbox,
    VTextarea,
    VTextField,
    VSelect,
    VForm,
    VBtn,
    VIcon,
} from 'vuetify/components';
import { useRouter } from 'vue-router';
import { CircleCheck } from 'lucide-vue-next';

import { useUsersStore } from '@/store/modules/usersStore';
import { useConfigStore } from '@/store/modules/configStore';
import { ROUTES } from '@/router';
import { MaxNameLengthRule, RequiredRule } from '@/types/common';
import { AnalyticsErrorEventSource, AnalyticsEvent } from '@/utils/constants/analyticsEventNames';
import { useLoading } from '@/composables/useLoading';
import { useNotify } from '@/utils/hooks';
import { useAnalyticsStore } from '@/store/modules/analyticsStore';

const router = useRouter();
const notify = useNotify();
const { isLoading, withLoading } = useLoading();

const configStore = useConfigStore();
const usersStore = useUsersStore();
const analyticsStore = useAnalyticsStore();

type FormData = {
    organization: string;
    industry: string | null;
    operatingSystem: string | null;
    teamSize: string | null;
    storageUsage: string | null;
    infrastructureType: string[];
    storageBackend: string[];
    mountSolution: string[];
    desiredFeatures: string[];
    painPoints: string[];
    comments: string;
};

const OtherLabel = 'Other';

const industries = [
    'Media & Entertainment',
    'Big Data',
    'Life Sciences',
    'High Performance Computing',
    'Machine Learning',
    OtherLabel,
];

const operatingSystems = ['macOS', 'Windows', 'Linux'];

const teamSizes = [
    'Individual',
    '2-5 people',
    '6-20 people',
    '21-50 people',
    '51-200 people',
    '200+ people',
];

const storageRanges = [
    'Less than 1 TB',
    '1TB-10TB',
    '10TB-50TB',
    '50TB-100TB',
    '100TB-1PB',
    '1PB-10PB',
    '10PB-50PB',
    '50PB+',
];

const infrastructureTypes = [
    'Cloud',
    'On-Premises',
    'Hybrid',
    'Multi-Cloud',
];

const storageBackends = [
    'Storj',
    'AWS S3',
    'Google Cloud Storage',
    'Azure Blob Storage',
    'Local NAS/SAN',
    'S3-Compatible Object Storage',
    OtherLabel,
];

const mountSolutions = [
    'LucidLink',
    'Mountain Duck',
    'NetApp Cloud Volumes',
    'None yet',
    OtherLabel,
];

const desiredFeatures = [
    'High Throughput',
    'Small File Performance',
    'Global Access',
    'Cloud Bursting',
    'POSIX Compatibility',
    'Direct Cloud Editing',
    'No File Downloads Required',
    'Intelligent Caching',
    'Multi-user Access',
    'Version Control',
    'High-speed Transfer',
    'Native Software Integration',
    'Lock Files',
    'SSO Integration',
    'Growing Files',
    'Pinning / Local caching',
];

const painPoints = [
    'Slow Download/Upload times',
    'File Version Conflicts',
    'Remote Collaboration Issues',
    'Limited Local Storage',
    'High Storage Costs',
    'Poor Performance with Large Files',
    'Complex Workflow Management',
];

enum State {
    Form,
    Success,
}

const formData = reactive<FormData>({
    organization: '',
    industry: null,
    operatingSystem: null,
    teamSize: null,
    storageUsage: null,
    infrastructureType: [],
    storageBackend: [],
    mountSolution: [],
    desiredFeatures: [],
    painPoints: [],
    comments: '',
});

const formValid = ref<boolean>(false);
const otherIndustry = ref<string>('');
const otherStorageBackend = ref<string>('');
const otherMountSolution = ref<string>('');

const userEmail = computed<string>(() => usersStore.state.user.email);
const betaJoined = computed<boolean>(() => usersStore.state.settings.noticeDismissal.cunoFSBetaJoined);

const state = ref<State>(betaJoined.value ? State.Success : State.Form);

function submitForm(): void {
    withLoading(async () => {
        if (!(formData.industry && formData.operatingSystem && formData.teamSize && formData.storageUsage)) return;

        try {
            const hubspotData = {
                companyName: formData.organization,
                industryUseCase: formData.industry,
                otherIndustryUseCase: otherIndustry.value,
                operatingSystem: formData.operatingSystem,
                teamSize: formData.teamSize,
                currentStorageUsage: formData.storageUsage,
                infraType: formData.infrastructureType.join(';'),
                currentStorageBackends: formData.storageBackend.join(';'),
                otherStorageBackend: otherStorageBackend.value,
                currentStorageMountSolution: formData.mountSolution.join(';'),
                otherStorageMountSolution: otherMountSolution.value,
                desiredFeatures: formData.desiredFeatures.join(';'),
                currentPainPoints: formData.painPoints.join(';'),
                specificTasks: formData.comments,
            };

            // This is a specific hubspot event tracking for cunoFS beta form submission.
            await analyticsStore.joinCunoFSBeta(hubspotData);

            const segmentProps = {
                organization: formData.organization,
                industry: formData.industry === OtherLabel ? otherIndustry.value : formData.industry,
                operatingSystem: formData.operatingSystem,
                teamSize: formData.teamSize,
                storageUsage: formData.storageUsage,
                infrastructureType: formData.infrastructureType.join(', '),
                storageBackend: formData.storageBackend.join(', '),
                mountSolution: formData.mountSolution.join(', '),
                desiredFeatures: formData.desiredFeatures.join(', '),
                painPoints: formData.painPoints.join(', '),
                comments: formData.comments,
            };
            if (formData.storageBackend.includes(OtherLabel) && otherStorageBackend.value) {
                segmentProps.storageBackend = segmentProps.storageBackend ? `${segmentProps.storageBackend}, ${otherStorageBackend.value}` : otherStorageBackend.value;
            }
            if (formData.mountSolution.includes(OtherLabel) && otherMountSolution.value) {
                segmentProps.mountSolution = segmentProps.mountSolution ? `${segmentProps.mountSolution}, ${otherMountSolution.value}` : otherMountSolution.value;
            }

            // This is a specific segment event tracking for cunoFS beta form submission.
            await analyticsStore.ensureEventTriggered(AnalyticsEvent.JOIN_CUNO_FS_BETA_FORM_SUBMITTED, segmentProps);

            const noticeDismissal = { ...usersStore.state.settings.noticeDismissal };
            noticeDismissal.cunoFSBetaJoined = true;
            await usersStore.updateSettings({ noticeDismissal });

            state.value = State.Success;
        } catch (error) {
            notify.notifyError(error, AnalyticsErrorEventSource.CUNO_FS_BETA_FORM);
        }
    });
}

watch(betaJoined, value => {
    if (value) state.value = State.Success;
});

onBeforeMount(() => {
    if (!configStore.state.config.cunoFSBetaEnabled) {
        router.push({ name: ROUTES.Dashboard.name });
    }
});
</script>
