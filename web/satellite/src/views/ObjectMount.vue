// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-container>
        <!-- Show success view if submitted, otherwise show form -->
        <v-row v-if="formSubmitted">
            <v-col>
                <v-card max-width="800" class="mx-auto pa-8 pt-5">
                    <v-sheet class="rounded-lg mb-5">
                        <v-avatar color="success" variant="flat" size="48" class="mt-2 mb-2">
                            <svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="text-white"><polyline points="20 6 9 17 4 12" /></svg>
                        </v-avatar>
                        <div class="text-overline font-weight-medium">SUBMISSION CONFIRMED</div>
                        <h2 class="text-h4 font-weight-medium">Request Received</h2>
                    </v-sheet>

                    <v-card-text class="pl-0 pt-0">
                        <p class="text-body-1">Thank you for your interest in Object Mount. Your request has been successfully submitted and assigned to our enterprise solutions team. We're committed to helping you transform your storage infrastructure with our high-performance mounting solution.</p>
                        <p class="text-body-1 mt-4">A member of our team will be in touch with you shortly to discuss your specific needs and arrange a personalized demonstration.</p>
                    </v-card-text>
                </v-card>
            </v-col>
        </v-row>

        <v-row v-else>
            <v-col>
                <v-card class="pa-4 pa-sm-8 mx-auto" max-width="800" elevation="2">
                    <!-- Header -->
                    <div class="d-flex align-center mb-6">
                        <h1 class="text-h4 mb-0">Object Mount</h1>
                    </div>

                    <!-- Hero section -->
                    <v-sheet color="grey-lighten-4" class="rounded-lg pa-6 mb-8 border">
                        <div class="d-flex flex-column flex-md-row">
                            <div class="flex-grow-1 pr-md-6">
                                <h3 class="mb-3">
                                    High-Performance File Access for Object Storage
                                </h3>
                                <p class="text-subtitle-1 mb-4">
                                    Transform your cloud storage into a high-performance local file system. Object Mount enables your team to work with S3-compatible remote files in real-time—without downloads or syncing—as if they were stored locally.
                                </p>
                                <v-btn color="primary" size="large" class="mt-2" href="#contact-form" variant="flat">
                                    Request Information
                                    <svg xmlns="http://www.w3.org/2000/svg" width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="ml-2"><polyline points="9 18 15 12 9 6" /></svg>
                                </v-btn>
                            </div>
                        </div>
                    </v-sheet>

                    <!-- Key benefits -->
                    <h3 class="text-h5 mb-4 d-flex align-center">
                        Key Benefits
                    </h3>

                    <v-row class="mb-8">
                        <v-col cols="12" md="4">
                            <v-card variant="outlined" class="h-100">
                                <v-card-item class="mt-1">
                                    <svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="lucide lucide-eye"><path d="M2.062 12.348a1 1 0 0 1 0-.696 10.75 10.75 0 0 1 19.876 0 1 1 0 0 1 0 .696 10.75 10.75 0 0 1-19.876 0" /><circle cx="12" cy="12" r="3" /></svg>                  <v-card-title class="px-0 font-weight-medium text-subtitle-1">Immediate Access</v-card-title>
                                    <v-card-text class="px-0 pt-2">
                                        Access files directly from your buckets with no download delays or storage duplication.
                                    </v-card-text>
                                </v-card-item>
                            </v-card>
                        </v-col>
                        <v-col cols="12" md="4">
                            <v-card variant="outlined" class="h-100">
                                <v-card-item class="mt-1">
                                    <svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="lucide lucide-hand-coins"><path d="M11 15h2a2 2 0 1 0 0-4h-3c-.6 0-1.1.2-1.4.6L3 17" /><path d="m7 21 1.6-1.4c.3-.4.8-.6 1.4-.6h4c1.1 0 2.1-.4 2.8-1.2l4.6-4.4a2 2 0 0 0-2.75-2.91l-4.2 3.9" /><path d="m2 16 6 6" /><circle cx="16" cy="9" r="2.9" /><circle cx="6" cy="5" r="3" /></svg>                  <v-card-title class="px-0 font-weight-medium text-subtitle-1">Cost Reduction</v-card-title>
                                    <v-card-text class="px-0 pt-2">
                                        Minimize egress fees by only retrieving data you actually need, when you need it.
                                    </v-card-text>
                                </v-card-item>
                            </v-card>
                        </v-col>
                        <v-col cols="12" md="4">
                            <v-card variant="outlined" class="h-100">
                                <v-card-item class="mt-1">
                                    <svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="lucide lucide-check-check"><path d="M18 6 7 17l-5-5" /><path d="m22 10-7.5 7.5L13 16" /></svg>                  <v-card-title class="px-0 font-weight-medium text-subtitle-1">Workflow Integration</v-card-title>
                                    <v-card-text class="px-0 pt-2">
                                        Works with your existing applications, allowing teams to maintain their preferred tools.
                                    </v-card-text>
                                </v-card-item>
                            </v-card>
                        </v-col>
                    </v-row>

                    <!-- Advanced features -->
                    <h3 class="text-h5 mb-4 d-flex align-center">
                        Advanced Features
                    </h3>

                    <v-row class="mb-8">
                        <v-col cols="12" md="4">
                            <v-card variant="outlined" class="h-100">
                                <v-card-item class="mt-1">
                                    <svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="lucide lucide-zap"><polygon points="13 2 3 14 12 14 11 22 21 10 12 10 13 2" /></svg>
                                    <v-card-title class="px-0 font-weight-medium text-subtitle-1">High-Performance</v-card-title>
                                    <v-card-text class="px-0 pt-2">
                                        Optimized for large files and media workflows with direct streaming capabilities.
                                    </v-card-text>
                                </v-card-item>
                            </v-card>
                        </v-col>
                        <v-col cols="12" md="4">
                            <v-card variant="outlined" class="h-100">
                                <v-card-item class="mt-1">
                                    <svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="lucide lucide-shuffle"><path d="M2 18h1.4c1.3 0 2.5-.6 3.3-1.7l6.1-8.6c.7-1.1 2-1.7 3.3-1.7H22" /><path d="m18 2 4 4-4 4" /><path d="M2 6h1.9c1.5 0 2.9.9 3.6 2.2" /><path d="M22 18h-5.9c-1.3 0-2.6-.7-3.3-1.8l-.5-.8" /><path d="m18 14 4 4-4 4" /></svg>
                                    <v-card-title class="px-0 font-weight-medium text-subtitle-1">Universal Compatibility</v-card-title>
                                    <v-card-text class="px-0 pt-2">
                                        Works with Storj buckets and any S3-compatible storage provider.
                                    </v-card-text>
                                </v-card-item>
                            </v-card>
                        </v-col>
                        <v-col cols="12" md="4">
                            <v-card variant="outlined" class="h-100">
                                <v-card-item class="mt-1">
                                    <svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="lucide lucide-database"><ellipse cx="12" cy="5" rx="9" ry="3" /><path d="M3 5v14c0 1.66 4 3 9 3s9-1.34 9-3V5" /><path d="M3 12c0 1.66 4 3 9 3s9-1.34 9-3" /></svg>
                                    <v-card-title class="px-0 font-weight-medium text-subtitle-1">Sophisticated Caching</v-card-title>
                                    <v-card-text class="px-0 pt-2">
                                        Intelligent data caching to optimize performance and reduce bandwidth usage.
                                    </v-card-text>
                                </v-card-item>
                            </v-card>
                        </v-col>
                    </v-row>

                    <!-- Contact form -->
                    <v-divider class="mb-8" />

                    <h3 id="contact-form" class="text-h5 mb-4 d-flex align-center">
                        <svg xmlns="http://www.w3.org/2000/svg" width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="lucide lucide-rocket mr-2"><path d="M4.5 16.5c-1.5 1.26-2 5-2 5s3.74-.5 5-2c.71-.84.7-2.13-.09-2.91a2.18 2.18 0 0 0-2.91-.09z" /><path d="m12 15-3-3a22 22 0 0 1 2-3.95A12.88 12.88 0 0 1 22 2c0 2.72-.78 7.5-6 11a22.35 22.35 0 0 1-4 2z" /><path d="M9 12H4s.55-3.03 2-4c1.62-1.08 5 0 5 0" /><path d="M12 15v5s3.03-.55 4-2c1.08-1.62 0-5 0-5" /></svg>
                        Start Your Object Mount Journey
                    </h3>

                    <p class="text-subtitle-1 mb-6">
                        Complete this form to receive a personalized demo and consultation. Our team will show you how Object Mount can transform your workflow and storage efficiency.
                    </p>

                    <v-form ref="form" v-model="valid" @submit.prevent="submitForm">
                        <v-text-field
                            v-model="formData.firstName"
                            label="Enter your first name"
                            variant="outlined"
                            required
                            :rules="[RequiredRule, MaxNameLengthRule]"
                            density="compact"
                            class="mb-3"
                        />

                        <v-text-field
                            v-model="formData.lastName"
                            label="Enter your last name"
                            variant="outlined"
                            required
                            :rules="[RequiredRule, MaxNameLengthRule]"
                            density="compact"
                            class="mb-3"
                        />

                        <v-text-field
                            v-model="formData.jobTitle"
                            label="Job Title"
                            variant="outlined"
                            density="compact"
                            required
                            :rules="[RequiredRule, MaxNameLengthRule]"
                            class="mb-3"
                        />

                        <v-text-field
                            v-model="formData.companyName"
                            label="Company Name"
                            variant="outlined"
                            density="compact"
                            required
                            :rules="[RequiredRule, MaxNameLengthRule]"
                            class="mb-3"
                        />

                        <v-text-field
                            v-model="user.email"
                            label="Email"
                            readonly
                            variant="outlined"
                            density="compact"
                            class="mb-3"
                        />

                        <v-text-field
                            v-model="formData.phone"
                            label="Phone Number"
                            variant="outlined"
                            density="compact"
                            type="tel"
                            :rules="[value => value === '' || PhoneNumberRule(value)]"
                            class="mb-3"
                        />

                        <v-select
                            v-model="formData.industry"
                            :items="industries"
                            label="Industry"
                            variant="outlined"
                            density="compact"
                            required
                            :rules="[RequiredRule]"
                            class="mb-3"
                        />

                        <v-select
                            v-model="formData.companySize"
                            :items="companySizes"
                            label="Company Size"
                            density="compact"
                            required
                            :rules="[RequiredRule]"
                            class="mb-3"
                        />

                        <v-textarea
                            v-model="formData.currentStorage"
                            label="Current Storage Solution"
                            placeholder="Please describe your current storage infrastructure"
                            variant="outlined"
                            class="mb-3"
                        />

                        <v-textarea
                            v-model="formData.challenges"
                            label="Key Challenges"
                            placeholder="Briefly explain the main challenges you are facing with your current storage or workflow"
                            variant="outlined"
                            class="mb-3"
                        />

                        <v-divider class="my-4" />
                        <p class="font-weight-medium d-flex align-center my-5">
                            Tell us about your Object Mount needs:
                        </p>

                        <v-select
                            v-model="formData.specificInterests"
                            :items="interestOptions"
                            label="Specific Interests"
                            multiple
                            density="compact"
                            class="mb-3"
                        />

                        <v-select
                            v-model="formData.storageNeeds"
                            :items="storageRanges"
                            density="compact"
                            label="Estimated Storage Needs"
                            required
                            :rules="[RequiredRule]"
                            class="mb-3"
                        />

                        <v-select
                            v-model="formData.timeline"
                            :items="timelineOptions"
                            label="Timeline for Implementation"
                            required
                            :rules="[RequiredRule]"
                            density="compact"
                            class="mb-3"
                        />

                        <v-textarea
                            v-model="formData.message"
                            label="Additional Information"
                            placeholder="Any additional comments or specific questions?"
                            variant="outlined"
                            hide-details="auto"
                            class="mb-3"
                        />

                        <v-checkbox
                            v-model="formData.agreement"
                            label="By submitting this form, you agree to be contacted by our team regarding Object Mount."
                            hide-details="auto"
                            required
                            :rules="[RequiredRule]"
                        />

                        <v-btn
                            type="submit"
                            color="primary"
                            size="large"
                            block
                            :loading="isLoading"
                            :disabled="!valid"
                            class="mt-3"
                        >
                            Get Your Personalized Demo
                            <svg xmlns="http://www.w3.org/2000/svg" width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="lucide lucide-arrow-right ml-2"><path d="M5 12h14" /><path d="m12 5 7 7-7 7" /></svg>
                        </v-btn>
                    </v-form>
                </v-card>
            </v-col>
        </v-row>
    </v-container>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onBeforeMount, watch } from 'vue';
import { useRouter } from 'vue-router';
import {
    VAvatar,
    VCard,
    VCardItem,
    VCardText,
    VCardTitle,
    VCheckbox,
    VCol,
    VContainer,
    VDivider,
    VForm,
    VRow,
    VSelect,
    VSheet,
    VTextField,
    VTextarea,
    VBtn,
} from 'vuetify/components';

import { useAnalyticsStore } from '@/store/modules/analyticsStore';
import { useNotify } from '@/composables/useNotify';
import { useUsersStore } from '@/store/modules/usersStore';
import { useConfigStore } from '@/store/modules/configStore';
import { useLoading } from '@/composables/useLoading';
import { ROUTES } from '@/router';
import { MaxNameLengthRule, PhoneNumberRule, RequiredRule } from '@/types/common';
import { AnalyticsErrorEventSource, AnalyticsEvent } from '@/utils/constants/analyticsEventNames';
import { User } from '@/types/users';

// Define states for the form
enum State {
    Form,
    Success,
}

const router = useRouter();
const { notifyError } = useNotify();
const { isLoading, withLoading } = useLoading();

const configStore = useConfigStore();
const usersStore = useUsersStore();
const analyticsStore = useAnalyticsStore();

const valid = ref(false);
const form = ref(null);
const state = ref<State>(State.Form);

// Check if the user has already submitted a consultation request
const consultationRequested = computed<boolean>(() =>
    usersStore.state.settings.noticeDismissal?.objectMountConsultationRequested || false);

// Watch for changes to the consultationRequested value
watch(consultationRequested, value => {
    if (value) state.value = State.Success;
});

onBeforeMount(() => {
    // Set initial state based on whether user has already submitted a request
    state.value = consultationRequested.value ? State.Success : State.Form;
});

// Computed property for form submission state
const formSubmitted = computed<boolean>(() => state.value === State.Success);

// Get user information
const user = computed<User>(() => usersStore.state.user);

// Define form data type
type FormData = {
    firstName: string;
    lastName: string;
    jobTitle: string;
    companyName: string;
    phone: string;
    industry: string | null;
    companySize: string | null;
    currentStorage: string;
    challenges: string;
    specificInterests: string[];
    storageNeeds: string | null;
    timeline: string | null;
    message: string;
    agreement: boolean;
};

// Initialize form data with user information where available
const formData = reactive<FormData>({
    firstName: user.value.fullName.split(' ')[0] ?? '',
    lastName: user.value.fullName.split(' ')[1] ?? '',
    jobTitle: '',
    companyName: '',
    phone: '',
    industry: null,
    companySize: null,
    currentStorage: '',
    challenges: '',
    specificInterests: [],
    storageNeeds: null,
    timeline: null,
    message: '',
    agreement: false,
});

const industries = [
    'Media & Entertainment',
    'Post Production',
    'Broadcasting',
    'VFX',
    'Advertising',
    'Scientific',
    'Big Data',
    'AI/ML',
    'Other',
];

const companySizes = [
    '1-10 employees',
    '11-50 employees',
    '51-200 employees',
    '201-500 employees',
    '500+ employees',
];

const interestOptions = [
    'Real-time Editing',
    'Cloud Collaboration',
    'Fast Ingest / Backup',
    'Browsing / Active Archive',
    'App Integration',
    'Other',
];

const timelineOptions = [
    'Within 1 month',
    '1-3 months',
    '3-6 months',
    '6+ months',
    'Not Yet Determined',
];

const storageRanges = [
    'Under 25 TB',
    '25 TB - 50 TB',
    '51 TB - 150 TB',
    '151 TB - 250 TB',
    '251 TB - 500 TB',
    '501 TB and above',
];

// Submit form function wrapped with loading state
function submitForm(): void {
    withLoading(async () => {
        if (!(formData.industry && formData.firstName && formData.lastName && formData.companyName && formData.jobTitle && formData.companySize && formData.storageNeeds && formData.agreement && formData.timeline)) return;

        try {
        // Prepare data for API
            const consultationData = {
                firstName: formData.firstName,
                lastName: formData.lastName,
                companyName: formData.companyName,
                jobTitle: formData.jobTitle,
                phoneNumber: formData.phone,
                industryUseCase: formData.industry,
                companySize: formData.companySize,
                currentStorageSolution: formData.currentStorage,
                keyChallenges: formData.challenges,
                specificInterests: Array.isArray(formData.specificInterests) ? formData.specificInterests.join(';') : '',
                storageNeeds: formData.storageNeeds,
                implementationTimeline: formData.timeline,
                additionalInformation: formData.message,
            };

            // Call the API endpoint
            await analyticsStore.requestObjectMountConsultation(consultationData);

            // Prepare segment properties for analytics
            const segmentProps = {
                firstName: formData.firstName,
                lastName: formData.lastName,
                companyName: formData.companyName,
                jobTitle: formData.jobTitle,
                phoneNumber: formData.phone,
                industryUseCase: formData.industry,
                companySize: formData.companySize,
                currentStorageSolution: formData.currentStorage,
                keyChallenges: formData.challenges,
                specificInterests: Array.isArray(formData.specificInterests) ? formData.specificInterests.join(', ') : '',
                storageNeeds: formData.storageNeeds,
                implementationTimeline: formData.timeline,
                additionalInformation: formData.message,
            };

            // Track successful form submission with detailed properties
            await analyticsStore.ensureEventTriggered(AnalyticsEvent.OBJECT_MOUNT_CONSULTATION_SUBMITTED, segmentProps);

            // Update user settings to remember form submission
            await usersStore.getSettings();

            // Update form state to show success view
            state.value = State.Success;
        } catch (error) {
            notifyError(error, AnalyticsErrorEventSource.OBJECT_MOUNT_CONSULTATION_FORM);
            console.error('Error submitting form:', error);
        }
    });
}

// Redirect to dashboard if feature is not enabled
onBeforeMount(() => {
    if (!configStore.state.config.objectMountConsultationEnabled) {
        router.push({ name: ROUTES.Dashboard.name });
    }
});

</script>
