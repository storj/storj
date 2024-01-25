// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

import { computed, onBeforeUnmount, ref } from 'vue';

import { AuthHttpApi } from '@/api/auth';
import { ErrorUnauthorized } from '@/api/errors/ErrorUnauthorized';
import { useConfigStore } from '@/store/modules/configStore';
import { useObjectBrowserStore } from '@/store/modules/objectBrowserStore';
import { useUsersStore } from '@/store/modules/usersStore';
import { useBucketsStore } from '@/store/modules/bucketsStore';
import { useProjectMembersStore } from '@/store/modules/projectMembersStore';
import { useABTestingStore } from '@/store/modules/abTestingStore';
import { useBillingStore } from '@/store/modules/billingStore';
import { useAccessGrantsStore } from '@/store/modules/accessGrantsStore';
import { useAppStore } from '@/store/modules/appStore';
import { useProjectsStore } from '@/store/modules/projectsStore';
import { useNotificationsStore } from '@/store/modules/notificationsStore';
import { AnalyticsErrorEventSource } from '@/utils/constants/analyticsEventNames';
import { useNotify } from '@/utils/hooks';
import { LocalData } from '@/utils/localData';

export interface UseSessionTimeoutOptions {
    showEditSessionTimeoutModal: () => void;
}

const RESET_ACTIVITY_EVENTS: readonly string[] = ['keypress', 'mousemove', 'mousedown', 'touchmove'];
export const INACTIVITY_MODAL_DURATION = 60000;

export function useSessionTimeout(opts: UseSessionTimeoutOptions) {
    const initialized = ref<boolean>(false);

    const inactivityTimerId = ref<ReturnType<typeof setTimeout> | null>(null);
    const sessionRefreshTimerId = ref<ReturnType<typeof setTimeout> | null>(null);
    const debugTimerId = ref<ReturnType<typeof setTimeout> | null>(null);
    const debugTimerText = ref<string>('');
    const isSessionActive = ref<boolean>(false);
    const isSessionRefreshing = ref<boolean>(false);
    const inactivityModalShown = ref<boolean>(false);
    const sessionExpiredModalShown = ref<boolean>(false);

    const configStore = useConfigStore();
    const bucketsStore = useBucketsStore();
    const pmStore = useProjectMembersStore();
    const usersStore = useUsersStore();
    const abTestingStore = useABTestingStore();
    const billingStore = useBillingStore();
    const agStore = useAccessGrantsStore();
    const appStore = useAppStore();
    const projectsStore = useProjectsStore();
    const notificationsStore = useNotificationsStore();
    const obStore = useObjectBrowserStore();

    const notify = useNotify();

    const auth: AuthHttpApi = new AuthHttpApi();

    /**
     * Returns the session duration from the store.
     */
    const sessionDuration = computed((): number => {
        const duration = (LocalData.getCustomSessionDuration() || usersStore.state.settings.sessionDuration?.fullSeconds || configStore.state.config.inactivityTimerDuration) * 1000;
        const maxTimeout = 2.1427e+9; // 24.8 days https://developer.mozilla.org/en-US/docs/Web/API/setTimeout#maximum_delay_value
        if (duration > maxTimeout) {
            return maxTimeout;
        }
        return duration;
    });

    /**
     * Returns the session refresh interval from the store.
     */
    const sessionRefreshInterval = computed((): number => {
        return Math.floor(sessionDuration.value * 0.75);
    });

    /**
     * Indicates whether to display the session timer for debugging.
     */
    const debugTimerShown = computed((): boolean => {
        return configStore.state.config.inactivityTimerViewerEnabled && initialized.value;
    });

    /**
     * Clears pinia stores and session timers, removes event listeners,
     * and displays the session expired modal.
     */
    async function clearStoresAndTimers(): Promise<void> {
        await Promise.all([
            pmStore.clear(),
            projectsStore.clear(),
            usersStore.clear(),
            agStore.stopWorker(),
            agStore.clear(),
            notificationsStore.clear(),
            bucketsStore.clear(),
            appStore.clear(),
            billingStore.clear(),
            abTestingStore.reset(),
            obStore.clear(),
        ]);

        RESET_ACTIVITY_EVENTS.forEach((eventName: string) => {
            document.removeEventListener(eventName, onSessionActivity);
        });
        LocalData.removeCustomSessionDuration();
        clearSessionTimers();
        inactivityModalShown.value = false;
        sessionExpiredModalShown.value = true;
    }

    /**
     * Performs logout and cleans event listeners and session timers.
     */
    async function handleInactive(): Promise<void> {
        await clearStoresAndTimers();

        try {
            await auth.logout();
        } catch (error) {
            if (error instanceof ErrorUnauthorized) return;

            notify.notifyError(error, AnalyticsErrorEventSource.OVERALL_SESSION_EXPIRED_ERROR);
        }
    }

    /**
     * Clears timers associated with session refreshing and inactivity.
     */
    function clearSessionTimers(): void {
        [inactivityTimerId.value, sessionRefreshTimerId.value, debugTimerId.value].forEach(id => {
            if (id !== null) clearTimeout(id);
        });
    }

    /**
     * Adds DOM event listeners and starts session timers.
     */
    async function setupSessionTimers(): Promise<void> {
        if (initialized.value || !configStore.state.config.inactivityTimerEnabled) return;

        const expiresAt = LocalData.getSessionExpirationDate();
        if (!expiresAt || expiresAt.getTime() - sessionDuration.value + sessionRefreshInterval.value < Date.now()) {
            await refreshSession();
        }

        RESET_ACTIVITY_EVENTS.forEach((eventName: string) => {
            document.addEventListener(eventName, onSessionActivity, false);
        });

        restartSessionTimers();

        initialized.value = true;
    }

    /**
     * Restarts timers associated with session refreshing and inactivity.
     */
    function restartSessionTimers(): void {
        sessionRefreshTimerId.value = setTimeout(async () => {
            sessionRefreshTimerId.value = null;
            if (isSessionActive.value) {
                await refreshSession();
            }
        }, sessionRefreshInterval.value);

        inactivityTimerId.value = setTimeout(async () => {
            if (obStore.uploadingLength) {
                await refreshSession();
                return;
            }

            if (isSessionActive.value) return;
            inactivityModalShown.value = true;
            inactivityTimerId.value = setTimeout(async () => {
                await clearStoresAndTimers();
                LocalData.setSessionHasExpired();
                notify.notify('Your session was timed out.');
            }, INACTIVITY_MODAL_DURATION);
        }, sessionDuration.value - INACTIVITY_MODAL_DURATION);

        if (!configStore.state.config.inactivityTimerViewerEnabled) return;

        const debugTimer = () => {
            const expiresAt = LocalData.getSessionExpirationDate();

            if (expiresAt) {
                const ms = Math.max(0, expiresAt.getTime() - Date.now());
                const secs = Math.floor(ms / 1000) % 60;

                debugTimerText.value = `${Math.floor(ms / 60000)}:${(secs < 10 ? '0' : '') + secs}`;

                if (ms > 1000) {
                    debugTimerId.value = setTimeout(debugTimer, 1000);
                }
            }
        };

        debugTimer();
    }

    /**
     * Refreshes session and resets session timers.
     * @param manual - whether the user manually refreshed session. i.e.: clicked "Stay Logged In".
     */
    async function refreshSession(manual = false): Promise<void> {
        isSessionRefreshing.value = true;

        try {
            LocalData.setSessionExpirationDate(await auth.refreshSession());
            if (LocalData.getCustomSessionDuration()) {
                LocalData.removeCustomSessionDuration();
            }
        } catch (error) {
            error.message = (error instanceof ErrorUnauthorized) ? 'Your session was timed out.' : error.message;
            notify.notifyError(error, AnalyticsErrorEventSource.ALL_PROJECT_DASHBOARD);
            await handleInactive();
            isSessionRefreshing.value = false;
            return;
        }

        clearSessionTimers();
        restartSessionTimers();
        inactivityModalShown.value = false;
        isSessionActive.value = false;
        isSessionRefreshing.value = false;

        if (manual && !usersStore.state.settings.sessionDuration) {
            opts.showEditSessionTimeoutModal();
        }
    }

    /**
     * Resets inactivity timer and refreshes session if necessary.
     */
    async function onSessionActivity(): Promise<void> {
        if (inactivityModalShown.value || isSessionActive.value) return;

        if (sessionRefreshTimerId.value === null && !isSessionRefreshing.value) {
            await refreshSession();
        }

        isSessionActive.value = true;
    }

    setupSessionTimers();

    usersStore.$onAction(({ name, after, args }) => {
        if (name === 'clear') clearSessionTimers();
        else if (name === 'updateSettings') {
            if (args[0].sessionDuration && args[0].sessionDuration !== usersStore.state.settings.sessionDuration?.nanoseconds) {
                after((_) => refreshSession());
            }
        }
    });

    onBeforeUnmount(() => {
        clearSessionTimers();
        RESET_ACTIVITY_EVENTS.forEach((eventName: string) => {
            document.removeEventListener(eventName, onSessionActivity);
        });
    });

    return {
        inactivityModalShown,
        sessionExpiredModalShown,
        debugTimerShown,
        debugTimerText,
        refreshSession,
        handleInactive,
        clearStoresAndTimers,
    };
}
