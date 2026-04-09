// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

import { computed, reactive } from 'vue';
import { defineStore } from 'pinia';

import {
    PlacementInfo,
    PlacementManagementHttpApiV1, ProductInfo, ProductManagementHttpApiV1,
    SearchHttpApiV1,
    SearchResult,
    Settings,
    SettingsHttpApiV1,
} from '@/api/client.gen';

export interface OIDCUser {
    email: string;
    groups: string[];
}

class AppState {
    public placements: PlacementInfo[];
    public products: ProductInfo[];
    public settings: Settings;
    public loading: boolean = false;
    public oidcUser: OIDCUser | null = null;
}

export const useAppStore = defineStore('app', () => {
    const state = reactive<AppState>(new AppState());

    const placementApi = new PlacementManagementHttpApiV1();
    const productApi = new ProductManagementHttpApiV1();
    const settingsApi = new SettingsHttpApiV1();
    const searchApi = new SearchHttpApiV1();

    const displayPlacements = computed<PlacementInfo[]>(() => {
        return state.placements.filter(p => !!p.location).map((p) => ({
            id: p.id,
            location: `${p.location} (${p.id})`,
        }));
    });

    const placementLocationMap = computed<Record<number, string>>(() => {
        const map: Record<number, string> = {};
        for (const placement of state.placements) {
            if (placement.location) {
                map[placement.id] = placement.location;
            }
        }
        return map;
    });

    async function load(fn : () => Promise<void>): Promise<void> {
        if (state.loading) return;
        state.loading = true;
        await fn();
        state.loading = false;
    }

    async function getPlacements(): Promise<void> {
        state.placements = await placementApi.getPlacements();
    }

    async function getProducts(): Promise<void> {
        state.products = await productApi.getProducts();
    }

    function getPlacementText(code: number): string {
        const location = placementLocationMap.value[code] ?? 'Unknown';
        return `${location} (${code})`;
    }

    function getPlacementID(location: string): number {
        for (const placement of state.placements) {
            if (placement.location === location) {
                return placement.id;
            }
        }
        return 0;
    }

    async function getSettings(): Promise<void> {
        state.settings = await settingsApi.get();
    }

    async function getOIDCSession(): Promise<void> {
        try {
            const response = await fetch('/auth/current-user');
            if (response.ok) {
                state.oidcUser = await response.json() as OIDCUser;
            }
        } catch { /* empty */ }
    }

    async function search(query: string): Promise<SearchResult> {
        return await searchApi.searchUsersProjectsOrNodes(query);
    }

    function logout(): void {
        window.location.href = '/auth/logout';
    }

    return {
        state,
        displayPlacements,
        load,
        getPlacements,
        getPlacementText,
        getPlacementID,
        getSettings,
        getOIDCSession,
        getProducts,
        search,
        logout,
    };
});
