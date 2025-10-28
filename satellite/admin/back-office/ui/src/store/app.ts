// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

import { reactive } from 'vue';
import { defineStore } from 'pinia';

import {
    PlacementInfo,
    PlacementManagementHttpApiV1, ProductInfo, ProductManagementHttpApiV1,
    SearchHttpApiV1,
    SearchResult,
    Settings,
    SettingsHttpApiV1,
} from '@/api/client.gen';

class AppState {
    public placements: PlacementInfo[];
    public products: ProductInfo[];
    public settings: Settings;
    public loading: boolean = false;
}

export const useAppStore = defineStore('app', () => {
    const state = reactive<AppState>(new AppState());

    const placementApi = new PlacementManagementHttpApiV1();
    const productApi = new ProductManagementHttpApiV1();
    const settingsApi = new SettingsHttpApiV1();
    const searchApi = new SearchHttpApiV1();

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
        for (const placement of state.placements) {
            if (placement.id === code) {
                if (placement.location) {
                    return placement.location;
                }
                break;
            }
        }
        return `Unknown (${code})`;
    }

    async function getSettings(): Promise<void> {
        state.settings = await settingsApi.get();
    }

    async function search(query: string): Promise<SearchResult> {
        return await searchApi.searchUsersOrProjects(query);
    }

    return {
        state,
        load,
        getPlacements,
        getPlacementText,
        getSettings,
        getProducts,
        search,
    };
});
