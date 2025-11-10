// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="audits-area">
        <div class="audits-area__content">
            <div v-for="item in auditItems" :key="item.satelliteName" class="audits-area__content__item">
                <div class="audits-area__content__item__header">
                    <p class="audits-area__content__item__header__satellite-name">{{ item.satelliteName }}</p>
                    <DisqualifyIcon v-if="item.iconClassName" :class="[ item.iconClassName ]" />
                </div>
                <div class="audits-area__content__item__divider" />
                <div class="audits-area__content__item__info">
                    <p class="audits-area__content__item__info__label">Suspension</p>
                    <p class="audits-area__content__item__info__value" :class="[ item.suspensionScore.statusClassName ]">{{ item.suspensionScore.label }}</p>
                </div>
                <div class="audits-area__content__item__info">
                    <p class="audits-area__content__item__info__label">Audit</p>
                    <p class="audits-area__content__item__info__value" :class="[ item.auditScore.statusClassName ]">{{ item.auditScore.label }}</p>
                </div>
                <div class="audits-area__content__item__info">
                    <p class="audits-area__content__item__info__label">Online</p>
                    <p class="audits-area__content__item__info__value" :class="[ item.onlineScore.statusClassName ]">{{ item.onlineScore.label }}</p>
                </div>
                <div class="audits-area__content__item__info">
                    <p class="audits-area__content__item__info__label">Vetted</p>
                    <p class="audits-area__content__item__info__value" :class="[ getVettedStatusClass(item.satelliteName) ]">{{ getVettedStatusLabel(item.satelliteName) }}</p>
                </div>
            </div>
        </div>
        <div v-if="isLoadMoreButtonVisible" class="audits-area__load-more-button" @click="loadMore">
            <p class="audits-area__load-more-button__label">Load More</p>
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import { SatelliteInfo, SatelliteScores } from '@/storagenode/sno/sno';

import DisqualifyIcon from '@/../static/images/disqualify.svg';

// @vue/component
@Component({
    components: { DisqualifyIcon },
})
export default class AllSatellitesAuditsArea extends Vue {
    /**
     * Number of score blocks displayed on page.
     */
    public numberOfItemsOnPage = 6;
    /**
     * Number of blocks added to displayed on page by clocking "Load more".
     */
    private readonly ITEMS_TO_ADD_COUNT: number = 6;

    /**
     * Returns reduced number of satellites score items depends on numberOfItemsOnPage.
     */
    public get auditItems(): SatelliteScores[] {
        return this.satellitesScores.slice(0, this.numberOfItemsOnPage);
    }

    /**
     * Indicates if all existing items are shown on page.
     */
    public get isLoadMoreButtonVisible(): boolean {
        return this.auditItems.length !== this.satellitesScores.length;
    }

    /**
     * Returns list of satellites score from store.
     */
    private get satellitesScores(): SatelliteScores[] {
        return this.$store.state.node.satellitesScores;
    }

    /**
     * Returns list of satellites from store.
     */
    private get satellites(): SatelliteInfo[] {
        return this.$store.state.node.satellites;
    }

    /**
     * Increments number of shown satellite score items by ITEMS_TO_ADD_COUNT.
     */
    public loadMore(): void {
        this.numberOfItemsOnPage += this.ITEMS_TO_ADD_COUNT;
    }

    /**
     * Returns vetted status label for satellite.
     */
    public getVettedStatusLabel(satelliteName: string): string {
        const satellite = this.findSatelliteByName(satelliteName);
        if (satellite?.vettedAt) {
            return satellite.vettedAt.toLocaleDateString();
        }
        return 'Not vetted';
    }

    /**
     * Returns vetted status CSS class for satellite.
     */
    public getVettedStatusClass(satelliteName: string): string {
        const satellite = this.findSatelliteByName(satelliteName);
        return satellite?.vettedAt ? 'vetted' : 'not-vetted';
    }

    /**
     * Finds satellite by name in satellites list.
     */
    private findSatelliteByName(satelliteName: string): SatelliteInfo | undefined {
        // SatelliteScores uses satelliteName but SatelliteInfo uses url
        // We need to match by URL since that's what's typically shown as the name
        return this.satellites.find(satellite => satellite.url === satelliteName);
    }
}
</script>

<style scoped lang="scss">
    .audits-area {
        display: flex;
        flex-direction: column;
        align-items: center;
        justify-content: center;

        &__content {
            width: 100%;
            display: grid;
            grid-gap: 15px;
            grid-template-columns: repeat(3, 1fr);

            &__item {
                padding: 12px 16px;
                display: flex;
                flex-direction: column;
                align-items: flex-start;
                justify-content: center;
                color: var(--regular-text-color);
                background-color: var(--block-background-color);
                font-size: 14px;
                border-radius: 10px;

                &__header {
                    display: flex;
                    align-items: center;
                    justify-content: space-between;
                    width: 100%;

                    &__satellite-name {
                        font-family: 'font_medium', sans-serif;
                    }
                }

                &__divider {
                    width: 100%;
                    background: #e5e9f1;
                    height: 1px;
                    margin: 12px 0;
                }

                &__info {
                    display: flex;
                    align-items: center;
                    justify-content: space-between;
                    width: 100%;

                    &__label {
                        font-family: 'font_regular', sans-serif;
                    }

                    &__value {
                        font-family: 'font_bold', sans-serif;
                        font-size: 16px;
                    }
                }
            }
        }

        &__load-more-button {
            display: flex;
            align-items: center;
            justify-content: center;
            width: 226px;
            height: 48px;
            border: 1px solid #afb7c1;
            box-sizing: border-box;
            border-radius: 8px;
            cursor: pointer;
            margin-top: 25px;

            &__label {
                font-family: 'font_medium', sans-serif;
                font-size: 16px;
                color: var(--regular-text-color);
            }
        }
    }

    .disqualification {
        color: var(--critical-color);

        :deep(path) {
            fill: var(--critical-color);
        }
    }

    .warning {
        color: var(--warning-color);

        :deep(path) {
            fill: var(--warning-color);
        }
    }

    .vetted {
        color: var(--success-color, #00bf5f);
    }

    .not-vetted {
        color: var(--warning-color);
    }

    @media screen and (max-width: 800px) {

        .audits-area__content {
            grid-template-columns: repeat(2, 1fr);
        }
    }

    @media screen and (max-width: 500px) {

        .audits-area__content {
            grid-template-columns: 1fr;
        }
    }
</style>
