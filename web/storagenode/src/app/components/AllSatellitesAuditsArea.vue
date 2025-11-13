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

<script setup lang="ts">
import { computed, ref } from 'vue';

import { SatelliteInfo, SatelliteScores } from '@/storagenode/sno/sno';
import { useNodeStore } from '@/app/store/modules/nodeStore';

import DisqualifyIcon from '@/../static/images/disqualify.svg';

const nodeStore = useNodeStore();

const ITEMS_TO_ADD_COUNT = 6;

const numberOfItemsOnPage = ref<number>(6);

const auditItems = computed<SatelliteScores[]>(()  => {
    return satellitesScores.value.slice(0, numberOfItemsOnPage.value);
});

const isLoadMoreButtonVisible = computed<boolean>(() => {
    return auditItems.value.length !== satellitesScores.value.length;
});

const satellitesScores = computed<SatelliteScores[]>(() => {
    return nodeStore.state.satellitesScores as SatelliteScores[];
});

const satellites = computed<SatelliteInfo[]>(() => {
    return nodeStore.state.satellites;
});

function loadMore(): void {
    numberOfItemsOnPage.value += ITEMS_TO_ADD_COUNT;
}

function getVettedStatusLabel(satelliteName: string): string {
    const satellite = findSatelliteByName(satelliteName);
    if (satellite?.vettedAt) {
        return satellite.vettedAt.toLocaleDateString();
    }
    return 'Not vetted';
}

function getVettedStatusClass(satelliteName: string): string {
    const satellite = findSatelliteByName(satelliteName);
    return satellite?.vettedAt ? 'vetted' : 'not-vetted';
}

function findSatelliteByName(satelliteName: string): SatelliteInfo | undefined {
    // SatelliteScores uses satelliteName but SatelliteInfo uses url
    // We need to match by URL since that's what's typically shown as the name
    return satellites.value.find(satellite => satellite.url === satelliteName);
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
            gap: 15px;
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

    @media screen and (width <= 800px) {

        .audits-area__content {
            grid-template-columns: repeat(2, 1fr);
        }
    }

    @media screen and (width <= 500px) {

        .audits-area__content {
            grid-template-columns: 1fr;
        }
    }
</style>
