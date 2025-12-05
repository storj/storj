// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

import { computed, onBeforeUnmount, ref } from 'vue';

import { AuthHttpApi } from '@/api/auth';
import { ErrorUnauthorized } from '@/api/errors/ErrorUnauthorized';
import { useConfigStore } from '@/store/modules/configStore';
import { useObjectBrowserStore } from '@/store/modules/objectBrowserStore';
import { useUsersStore } from '@/store/modules/usersStore';
import { AnalyticsErrorEventSource } from '@/utils/constants/analyticsEventNames';
import { useNotify } from '@/composables/useNotify';
import { LocalData } from '@/utils/localData';
import { useLogout } from '@/composables/useLogout';

export interface UseSessionTimeoutOptions {
    showEditSessionTimeoutModal: () => void;
}

// Events that indicate user activity and should reset the inactivity timer.
const RESET_ACTIVITY_EVENTS: readonly string[] = ['keypress', 'mousemove', 'mousedown', 'touchmove'];

// Duration in milliseconds for which to show the inactivity warning before logout.
export const INACTIVITY_MODAL_DURATION = 60000;

export function useSessionTimeout(opts: UseSessionTimeoutOptions) {
    // Flag to prevent double initialization.
    const initialized = ref<boolean>(false);

    const inactivityTimerId = ref<NodeJS.Timeout>();
    const sessionRefreshTimerId = ref<NodeJS.Timeout>();
    const debugTimerId = ref<NodeJS.Timeout>();

    const debugTimerText = ref<string>('');

    // Indicates if user has shown activity recently.
    const isUserActive = ref<boolean>(false);
    // Indicates if session refresh is in progress.
    const isSessionRefreshing = ref<boolean>(false);
    const inactivityModalShown = ref<boolean>(false);
    const sessionExpiredModalShown = ref<boolean>(false);

    const configStore = useConfigStore();
    const usersStore = useUsersStore();
    const obStore = useObjectBrowserStore();

    const notify = useNotify();
    const { clearStores } = useLogout();

    const auth: AuthHttpApi = new AuthHttpApi();

    /**
     * Returns the session duration from the store.
     * Prioritizes custom duration > user settings > global config.
     * Ensures the duration doesn't exceed setTimeout's maximum value.
     */
    const sessionDuration = computed((): number => {
        const duration = (
            LocalData.getCustomSessionDuration() ||
            usersStore.state.settings.sessionDuration?.fullSeconds ||
            configStore.state.config.inactivityTimerDuration
        ) * 1000;

        // Maximum value setTimeout can handle (approximately 24.8 days).
        const maxTimeout = 2.1427e+9;

        if (duration > maxTimeout) {
            return maxTimeout;
        }
        return duration;
    });

    /**
     * Returns the session refresh interval - typically 75% of session duration.
     * This ensures sessions are refreshed before they expire.
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
     * Clears all state and timers when session has expired or user is logged out.
     * - Clears Pinia stores
     * - Removes event listeners
     * - Clears all timers
     * - Shows the session expired modal
     */
    async function clearStoresAndTimers(): Promise<void> {
        await clearStores();

        RESET_ACTIVITY_EVENTS.forEach((eventName: string) => {
            document.removeEventListener(eventName, onSessionActivity);
        });

        LocalData.removeCustomSessionDuration();
        clearSessionTimers();
        inactivityModalShown.value = false;
        sessionExpiredModalShown.value = true;
    }

    /**
     * Handles user inactivity by logging them out and cleaning up.
     * Called when inactivity timeout is reached.
     */
    async function handleInactive(): Promise<void> {
        await clearStoresAndTimers();

        try {
            await auth.logout(configStore.state.config.csrfToken);
        } catch (error) {
            if (error instanceof ErrorUnauthorized || error.status === 403) return; // 403 comes from no CSRF cookie.

            notify.notifyError(error, AnalyticsErrorEventSource.OVERALL_SESSION_EXPIRED_ERROR);
        }
    }

    /**
     * Clears all timers related to session management.
     */
    function clearSessionTimers(): void {
        [inactivityTimerId.value, sessionRefreshTimerId.value, debugTimerId.value].forEach(id => {
            if (id !== undefined) clearTimeout(id);
        });

        inactivityTimerId.value = undefined;
        sessionRefreshTimerId.value = undefined;
        debugTimerId.value = undefined;
    }

    /**
     * Sets up event listeners and initializes session timers.
     * Called when component is mounted.
     */
    async function setupSessionTimers(): Promise<void> {
        // Skip if already initialized or if inactivity timer is disabled in config.
        if (initialized.value || !configStore.state.config.inactivityTimerEnabled) return;

        const expiresAt = LocalData.getSessionExpirationDate();

        // Refresh if:
        // 1. No expiration date exists, or
        // 2. The session is close to expiring (less than 25% of session duration remains)
        if (!expiresAt || (expiresAt.getTime() - Date.now()) < sessionRefreshInterval.value) {
            await refreshSession();
        }

        RESET_ACTIVITY_EVENTS.forEach((eventName: string) => {
            document.addEventListener(eventName, onSessionActivity, false);
        });

        restartSessionTimers();

        initialized.value = true;
    }

    /**
     * Restarts all timers related to session management.
     * - Session refresh timer: refreshes session when 75% of duration has passed
     * - Inactivity timer: checks for inactivity when session is about to expire
     * - Debug timer (optional): updates the visual countdown
     */
    function restartSessionTimers(): void {
        // Session refresh timer - refreshes session at 75% of total duration.
        sessionRefreshTimerId.value = setTimeout(async () => {
            sessionRefreshTimerId.value = undefined;

            // Refresh session if there was activity or uploads are in progress.
            if (isUserActive.value || obStore.uploadingLength > 0) {
                // Reset activity flag only if it was active.
                if (isUserActive.value) isUserActive.value = false;

                await refreshSession();
            }
        }, sessionRefreshInterval.value);

        // Inactivity timer - shows warning before session expires.
        inactivityTimerId.value = setTimeout(async () => {
            // If there are uploads, always refresh regardless of activity.
            if (obStore.uploadingLength > 0) {
                await refreshSession();
                return;
            }

            // Check if there was activity since the last refresh.
            if (isUserActive.value) {
                isUserActive.value = false;
                await refreshSession();
                return;
            }

            // No activity and no uploads, show inactivity modal.
            inactivityModalShown.value = true;

            // Set final timeout before logging out.
            inactivityTimerId.value = setTimeout(async () => {
                await clearStoresAndTimers();
                LocalData.setSessionHasExpired();
                notify.notify('Your session was timed out.');
            }, INACTIVITY_MODAL_DURATION);
        }, sessionDuration.value - INACTIVITY_MODAL_DURATION);

        // Debug timer (optional feature) - shows countdown display.
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
     * Refreshes the session with the server and resets all timers.
     * Called periodically or when user manually refreshes session.
     *
     * @param manual - whether the user manually refreshed session (e.g. clicked "Stay Logged In")
     */
    async function refreshSession(manual = false): Promise<void> {
        // Prevent concurrent refreshes.
        if (isSessionRefreshing.value) return;

        isSessionRefreshing.value = true;

        try {
            LocalData.setSessionExpirationDate(await auth.refreshSession(configStore.state.config.csrfToken));

            // Clear custom duration if set (will revert to user settings).
            if (LocalData.getCustomSessionDuration()) {
                LocalData.removeCustomSessionDuration();
            }

            clearSessionTimers();
            restartSessionTimers();

            inactivityModalShown.value = false;

            if (isUserActive.value) isUserActive.value = false;

            if (manual && !usersStore.state.settings.sessionDuration) {
                opts.showEditSessionTimeoutModal();
            }
        } catch (error) {
            error.message = (error instanceof ErrorUnauthorized)
                ? 'Your session was timed out.'
                : error.message;

            notify.notifyError(error, AnalyticsErrorEventSource.OVERALL_SESSION_EXPIRED_ERROR);
            await handleInactive();
        } finally {
            isSessionRefreshing.value = false;
        }
    }

    /**
     * Event handler for user activity events.
     * Marks the session as active and refreshes if needed.
     */
    async function onSessionActivity(): Promise<void> {
        if (inactivityModalShown.value || sessionExpiredModalShown.value || isUserActive.value) return;

        // Mark that user is active.
        isUserActive.value = true;

        // If refresh timer is not running and we're not already refreshing,
        // perform an immediate refresh (useful if session is close to expiring).
        if (sessionRefreshTimerId.value === undefined && !isSessionRefreshing.value) {
            await refreshSession();
        }
    }

    // Initialize timers when this composable is used.
    setupSessionTimers();

    // Watch for actions on the users store that might affect session.
    usersStore.$onAction(({ name, after, args }) => {
        // Clear timers when user store is cleared (logout).
        if (name === 'clear') clearSessionTimers();
        // Refresh session when session duration setting is updated.
        else if (name === 'updateSettings') {
            if (args[0].sessionDuration &&
                args[0].sessionDuration !== usersStore.state.settings.sessionDuration?.nanoseconds) {
                after((_) => refreshSession());
            }
        }
    });

    // Clean up when component is unmounted.
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
    };
}
