// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <branded-loader v-if="isLoading" />
    <ErrorPage v-else-if="isErrorPageShown" />
    <template v-else>
        <router-view />
        <trial-expiration-dialog
            v-if="!user.isPaid"
            v-model="appStore.state.isExpirationDialogShown"
            :expired="user.freezeStatus.trialExpiredFrozen"
        />
        <managed-passphrase-error-dialog
            v-if="appStore.state.managedPassphraseErrorDialogShown"
            v-model="appStore.state.managedPassphraseErrorDialogShown"
        />
    </template>
    <Notifications />
</template>

<script setup lang="ts">
import { computed, onBeforeMount, onBeforeUnmount, ref, watch } from 'vue';
import { useRoute, useRouter } from 'vue-router';

import { useConfigStore } from '@/store/modules/configStore';
import { useAppStore } from '@/store/modules/appStore';
import { APIError, ObjectDeleteError } from '@/utils/error';
import { ErrorUnauthorized } from '@/api/errors/ErrorUnauthorized';
import { useABTestingStore } from '@/store/modules/abTestingStore';
import { useUsersStore } from '@/store/modules/usersStore';
import { useProjectsStore } from '@/store/modules/projectsStore';
import { useAnalyticsStore } from '@/store/modules/analyticsStore';
import { useNotify } from '@/composables/useNotify';
import { useBillingStore } from '@/store/modules/billingStore';
import { AnalyticsErrorEventSource, AnalyticsEvent } from '@/utils/constants/analyticsEventNames';
import { ROUTES } from '@/router';
import { User } from '@/types/users';
import { useBucketsStore } from '@/store/modules/bucketsStore';
import { PricingPlanInfo } from '@/types/common';
import { EdgeCredentials } from '@/types/accessGrants';
import { useObjectBrowserStore } from '@/store/modules/objectBrowserStore';
import { ProjectConfig } from '@/types/projects';
import { PaymentStatus, PaymentWithConfirmations } from '@/types/payments';
import { DARK_THEME_QUERY, useThemeStore } from '@/store/modules/themeStore';

import Notifications from '@/layouts/default/Notifications.vue';
import ErrorPage from '@/components/ErrorPage.vue';
import BrandedLoader from '@/components/utils/BrandedLoader.vue';
import TrialExpirationDialog from '@/components/dialogs/TrialExpirationDialog.vue';
import ManagedPassphraseErrorDialog from '@/components/dialogs/ManagedPassphraseErrorDialog.vue';

const appStore = useAppStore();
const abTestingStore = useABTestingStore();
const billingStore = useBillingStore();
const bucketsStore = useBucketsStore();
const configStore = useConfigStore();
const themeStore = useThemeStore();
const usersStore = useUsersStore();
const obStore = useObjectBrowserStore();
const projectsStore = useProjectsStore();
const analyticsStore = useAnalyticsStore();

const notify = useNotify();
const router = useRouter();
const route = useRoute();

const isLoading = ref<boolean>(true);

const darkThemeMediaQuery = window.matchMedia(DARK_THEME_QUERY);

/**
 * Returns pending payments from store.
 */
const pendingPayments = computed<PaymentWithConfirmations[]>(() => {
    return billingStore.state.pendingPaymentsWithConfirmations;
});

/**
 * Indicates whether an error page should be shown in place of the router view.
 */
const isErrorPageShown = computed<boolean>((): boolean => {
    return appStore.state.error.visible;
});

/**
 * Returns user entity from store.
 */
const user = computed<User>(() => usersStore.state.user);

/**
 * Determine whether the current user is eligible for pricing plans.
 */
async function getPricingPlansAvailable() {
    if (!configStore.getBillingEnabled(usersStore.state.user)
        || !configStore.state.config.pricingPackagesEnabled) {
        return;
    }
    const user: User = usersStore.state.user;
    if (user.hasPaidPrivileges || !user.partner) {
        return;
    }

    try {
        const hasPkg = await billingStore.getPricingPackageAvailable();
        if (!hasPkg) {
            return;
        }
    } catch (error) {
        notify.notifyError(error, AnalyticsErrorEventSource.OVERALL_APP_WRAPPER_ERROR);
        return;
    }

    let config;
    try {
        config = (await import('@/configs/pricingPlanConfig.json')).default;
    } catch {
        return;
    }

    const info = (config[user.partner] as PricingPlanInfo);
    if (!info) {
        notify.error(`No pricing plan configuration for partner '${user.partner}'.`, null);
        return;
    }
    billingStore.setPricingPlansAvailable(true, info);
}

/**
 * Sets up the app by fetching all necessary data.
 */
async function setup() {
    isLoading.value = true;
    const source = new URLSearchParams(window.location.search).get('source');
    try {
        await usersStore.getUser();
        const promises: Promise<void | object | string>[] = [
            usersStore.getSettings(),
            projectsStore.getProjects(),
            projectsStore.getUserInvitations(),
            abTestingStore.fetchValues(),
        ];
        if (configStore.state.config.billingFeaturesEnabled) {
            promises.push(billingStore.setupAccount().catch((e) => {
                notify.notifyError(e, AnalyticsErrorEventSource.OVERALL_APP_WRAPPER_ERROR);
            }));
            promises.push(getPricingPlansAvailable());
        }
        await Promise.all(promises);

        const invites = projectsStore.state.invitations;
        const projects = projectsStore.state.projects;

        if (source) {
            analyticsStore.eventTriggered(AnalyticsEvent.ARRIVED_FROM_SOURCE, { source: source });
        }

        if (appStore.state.hasJustLoggedIn && !invites.length && projects.length === 1) {
            projectsStore.selectProject(projects[0].id);
            const project = projectsStore.state.selectedProject;
            await router.push({
                name: ROUTES.Dashboard.name,
                params: { id: project.urlId },
            });
            analyticsStore.eventTriggered(AnalyticsEvent.NAVIGATE_PROJECTS);
        }
    } catch (error) {
        if (!(error instanceof ErrorUnauthorized)) {
            notify.notifyError(error, AnalyticsErrorEventSource.OVERALL_APP_WRAPPER_ERROR);
            appStore.setErrorPage((error as APIError).status ?? 500, true);
        } else {
            await new Promise(resolve => setTimeout(resolve, 1000));
            if (!ROUTES.AuthRoutes.includes(route.path)) await router.push(ROUTES.Login.path);
        }
    }
    isLoading.value = false;
}

function totalValueCounter(list: PaymentWithConfirmations[], field: keyof PaymentWithConfirmations): number {
    return list.reduce((acc: number, curr: PaymentWithConfirmations) => {
        return acc + (curr[field] as number);
    }, 0);
}

function onThemeChange(e: MediaQueryListEvent) {
    themeStore.setThemeLightness(!e.matches);
}

/**
 * Lifecycle hook before initial render.
 * Sets up variables from meta tags from config such satellite name, etc.
 */
onBeforeMount(async (): Promise<void> => {
    try {
        await configStore.getConfig();
    } catch (error) {
        isLoading.value = false;
        notify.notifyError(error, AnalyticsErrorEventSource.OVERALL_APP_WRAPPER_ERROR);
        appStore.setErrorPage((error as APIError).status ?? 500, true);
        return;
    }

    await setup();

    isLoading.value = false;

    if (configStore.state.config.analyticsEnabled) {
        analyticsStore.pageVisit(route.matched[route.matched.length - 1].path, configStore.state.config.satelliteName);
    }
});

usersStore.$onAction(({ name, after }) => {
    if (name === 'login') {
        after((_) => {
            setup().then(() => {
                if (user.value.hasPaidPrivileges || route.name !== ROUTES.Dashboard.name || projectsStore.state.selectedProject.ownerId !== user.value.id) return;

                const expirationInfo = user.value.getExpirationInfo(configStore.state.config.daysBeforeTrialEndNotification);
                if (user.value.freezeStatus.trialExpiredFrozen || expirationInfo.isCloseToExpiredTrial) {
                    appStore.toggleExpirationDialog(true);
                }
            });
        });
    }
});

bucketsStore.$onAction(({ name, after, args }) => {
    if (name === 'handleDeleteBucketRequest') {
        after(async (_) => {
            const bucketName = args[0];
            const request = args[1];
            try {
                await request;
                analyticsStore.eventTriggered(AnalyticsEvent.BUCKET_DELETED, { project_id: projectsStore.state.selectedProject.id });
                notify.success(`Successfully deleted ${bucketName}.`, 'Bucket Deleted');
            } catch (error) {
                let message = `Failed to delete ${bucketName}.`;
                if (error && error.message) {
                    message += ` ${error.message}`;
                }
                notify.error(message, AnalyticsErrorEventSource.OVERALL_APP_WRAPPER_ERROR);
            }

            try {
                await bucketsStore.getBuckets(1, projectsStore.state.selectedProject.id, bucketsStore.state.cursor.limit);
            } catch (error) {
                notify.notifyError(error, AnalyticsErrorEventSource.OVERALL_APP_WRAPPER_ERROR);
            }
        });
    }
});

obStore.$onAction(({ name, after, args }) => {
    if (name === 'handleDeleteObjectRequest') {
        after(async (_) => {
            const request = args[0];
            let label = args[1] ?? 'object';
            let deletedCount = 0;
            try {
                deletedCount = await request;
            } catch (error) {
                error.message = `Deleting failed. ${error.message}`;
                notify.notifyError(error, AnalyticsErrorEventSource.OVERALL_APP_WRAPPER_ERROR);
                if (error instanceof ObjectDeleteError) {
                    deletedCount = error.deletedCount;
                } else {
                    obStore.filesDeleted();
                    return;
                }
            }

            if (deletedCount) {
                label = deletedCount > 1 ? `${label}s` : label;
                notify.success(`${deletedCount} ${label} deleted`);
                obStore.filesDeleted();
            }
        });
    }
});

const processedTXs = new Set<string>();

watch(pendingPayments, async newPayments => {
    if (!newPayments.length) return;

    const unprocessedConfirmedPayments = newPayments.filter(p => p.status === PaymentStatus.Confirmed && !processedTXs.has(p.transaction));
    if (!unprocessedConfirmedPayments.length) return;

    const tokensSum = totalValueCounter(unprocessedConfirmedPayments, 'tokenValue');
    if (tokensSum > 0) {
        const bonusSum = totalValueCounter(unprocessedConfirmedPayments, 'bonusTokens');

        notify.success(`Successful deposit of ${tokensSum} STORJ tokens. You received an additional bonus of ${bonusSum} STORJ tokens.`);

        Promise.all([
            billingStore.getBalance(),
            billingStore.getWallet(),
        ]).catch((_) => {});
    }

    unprocessedConfirmedPayments.forEach(p => processedTXs.add(p.transaction));

    if (user.value.isFree) {
        await usersStore.getUser();

        if (user.value.isPaid) {
            await Promise.all([
                projectsStore.getProjectConfig(),
                projectsStore.getProjectLimits(projectsStore.state.selectedProject.id),
            ]).catch((_) => {});

            notify.success('Account was successfully upgraded!');
        }
    }
}, { deep: true, immediate: true });

/**
 * reset pricing plans available when user upgrades to paid tier.
 */
watch(() => user.value.hasPaidPrivileges, (paidTier) => {
    if (paidTier) {
        billingStore.setPricingPlansAvailable(false, null);
    }
});

/**
 * conditionally prompt for project passphrase if project changes
 */
watch(() => projectsStore.state.selectedProject, async (project, oldProject) => {
    if (!project.id || project.id === oldProject.id) {
        return;
    }
    try {
        appStore.setManagedPassphraseNotRetrievable(false);
        const results = await Promise.all([
            projectsStore.getProjectLimits(project.id),
            projectsStore.getProjectConfig(),
        ]);
        const config = results[1] as ProjectConfig;
        if (config.hasManagedPassphrase && config.passphrase) {
            bucketsStore.setEdgeCredentials(new EdgeCredentials());
            bucketsStore.setPassphrase(config.passphrase);
            bucketsStore.setPromptForPassphrase(false);
            return;
        } else if (config.hasManagedPassphrase) { // satellite failed to provide decrypted passphrase
            appStore.setManagedPassphraseNotRetrievable(true);
            throw new Error('Unable to acquire managed encryption passphrase');
        } else if (
            usersStore.getShouldPromptPassphrase(project.ownerId === usersStore.state.user.id) &&
            !user.value.freezeStatus.trialExpiredFrozen &&
            route.name !== ROUTES.Bucket.name
        ) {
            appStore.toggleProjectPassphraseDialog(true);
        }
    } catch (error) {
        notify.notifyError(error, AnalyticsErrorEventSource.OVERALL_APP_WRAPPER_ERROR);
    }
});

watch(() => themeStore.state.name, (theme) => {
    if (theme === 'auto') {
        darkThemeMediaQuery.addEventListener('change', onThemeChange);
        return;
    }
    darkThemeMediaQuery.removeEventListener('change', onThemeChange);
}, { immediate: true });

onBeforeUnmount(() => {
    billingStore.stopPaymentsPolling();
    darkThemeMediaQuery.removeEventListener('change', onThemeChange);
});
</script>
