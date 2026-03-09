// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-overlay v-model="model" persistent />

    <v-dialog
        :model-value="model && !isUpgradeDialogShown"
        width="410px"
        transition="fade-transition"
        :persistent="isDialogPersistent"
        :scrim="false"
        scrollable
        @update:model-value="v => model = v"
    >
        <v-card>
            <account-frozen-view
                v-if="isFrozen"
                @cancel="model = false"
            />
            <upgrade-required-view
                v-else-if="isMemberAccount"
                @cancel="model = false"
                @show-upgrade="openUpgradeDialog(true)"
            />
            <project-limit-reached-view
                v-else-if="isProjectLimitReached"
                ref="limitView"
                @cancel="model = false"
                @show-upgrade="openUpgradeDialog(false)"
                @update:loading="v => isLoading = v"
            />
            <create-project-form-view
                v-else
                ref="formView"
                @cancel="model = false"
                @created="onProjectCreated"
                @update:loading="v => isLoading = v"
            />
        </v-card>
    </v-dialog>

    <upgrade-account-dialog
        :scrim="false"
        :is-member-upgrade="isMemberUpgradeContext"
        :model-value="model && isUpgradeDialogShown"
        @update:model-value="v => model = isUpgradeDialogShown = v"
        @member-upgrade="model = true"
    />
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue';
import { useRouter } from 'vue-router';
import { VCard, VDialog, VOverlay } from 'vuetify/components';

import { Project } from '@/types/projects';
import { useProjectsStore } from '@/store/modules/projectsStore';
import { useUsersStore } from '@/store/modules/usersStore';
import { useConfigStore } from '@/store/modules/configStore';
import { useNotify } from '@/composables/useNotify';
import { ROUTES } from '@/router';

import UpgradeAccountDialog from '@/components/dialogs/upgradeAccountFlow/UpgradeAccountDialog.vue';
import AccountFrozenView from '@/components/dialogs/createProject/AccountFrozenView.vue';
import UpgradeRequiredView from '@/components/dialogs/createProject/UpgradeRequiredView.vue';
import ProjectLimitReachedView from '@/components/dialogs/createProject/ProjectLimitReachedView.vue';
import CreateProjectFormView from '@/components/dialogs/createProject/CreateProjectFormView.vue';

const model = defineModel<boolean>({ required: true });

const projectsStore = useProjectsStore();
const usersStore = useUsersStore();
const configStore = useConfigStore();

const notify = useNotify();
const router = useRouter();

const isUpgradeDialogShown = ref(false);
const isMemberUpgradeContext = ref(false);
const isLoading = ref(false);

const limitView = ref<InstanceType<typeof ProjectLimitReachedView>>();
const formView = ref<InstanceType<typeof CreateProjectFormView>>();

const isFrozen = computed(() =>
    usersStore.state.user.freezeStatus.frozen || usersStore.state.user.freezeStatus.trialExpiredFrozen,
);

const billingEnabled = computed(() => configStore.getBillingEnabled(usersStore.state.user));

const isMemberAccount = computed(() => usersStore.state.user.isMember && billingEnabled.value);

const isProjectLimitReached = ref(false);

const isDialogPersistent = computed(() =>
    isLoading.value || (!configStore.state.config.hideProjectEncryptionOptions && configStore.state.config.satelliteManagedEncryptionEnabled),
);

function openUpgradeDialog(isMemberUpgrade: boolean): void {
    isMemberUpgradeContext.value = isMemberUpgrade;
    isUpgradeDialogShown.value = true;
}

function onProjectCreated(project: Project): void {
    model.value = false;
    router.push({
        name: ROUTES.Dashboard.name,
        params: { id: project.urlId },
    });
    notify.success('Project created.');
}

watch(model, val => {
    if (val) {
        const ownedProjects = projectsStore.projects.filter(p => p.ownerId === usersStore.state.user.id);
        isProjectLimitReached.value = ownedProjects.length >= usersStore.state.user.projectLimit && billingEnabled.value;
    } else {
        isLoading.value = false;
        isUpgradeDialogShown.value = false;
        // Delay reset to allow dialog close animation to finish and avoid jank.
        setTimeout(() => {
            limitView.value?.reset();
            formView.value?.reset();
        }, 500);
    }
});

watch(isUpgradeDialogShown, val => {
    if (!val && !isMemberUpgradeContext.value && usersStore.state.user.isPaid) {
        model.value = true;
    }
});
</script>
