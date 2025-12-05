// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="info-area">
        <SatelliteSelection />
        <div v-if="isDisqualifiedInfoShown" class="info-area__disqualified-info">
            <LargeDisqualificationIcon
                class="info-area__disqualified-info__image"
                alt="Disqualified image"
            />
            <p class="info-area__disqualified-info__info">
                Your node has been disqualified on <b>{{ getDisqualificationDate }}</b>. If you have any questions regarding this please check our Node Operators
                <a
                    class="info-area__disqualified-info__info__link"
                    href="https://forum.storj.io/c/sno-category"
                    rel="noopener noreferrer"
                    target="_blank"
                >
                    thread
                </a> on Storj forum.
            </p>
        </div>
        <div v-else-if="doDisqualifiedSatellitesExist" class="info-area__disqualified-info">
            <LargeDisqualificationIcon
                class="info-area__disqualified-info__image"
                alt="Disqualified image"
            />
            <p class="info-area__disqualified-info__info">
                Your node has been disqualified on<span v-for="disqualified in disqualifiedSatellites" :key="disqualified.id"><b> {{ disqualified.id }}</b></span>. If you have any questions regarding this please check our Node Operators
                <a
                    class="info-area__disqualified-info__info__link"
                    href="https://forum.storj.io/c/sno-category"
                    rel="noopener noreferrer"
                    target="_blank"
                >
                    thread
                </a> on Storj forum.
            </p>
        </div>
        <div v-if="isSuspendedInfoShown" class="info-area__suspended-info">
            <LargeSuspensionIcon
                class="info-area__suspended-info__image"
                alt="Suspended image"
            />
            <p class="info-area__suspended-info__info">
                Your node has been suspended on <b>{{ getSuspensionDate }}</b>. If you have any questions regarding this please check our Node Operators
                <a
                    class="info-area__disqualified-info__info__link"
                    href="https://forum.storj.io/c/sno-category"
                    rel="noopener noreferrer"
                    target="_blank"
                >
                    thread
                </a> on Storj forum.
            </p>
        </div>
        <div v-else-if="doSuspendedSatellitesExist" class="info-area__suspended-info">
            <LargeSuspensionIcon
                class="info-area__suspended-info__image"
                alt="Suspended image"
            />
            <p class="info-area__suspended-info__info">
                Your node has been suspended on<span v-for="suspended in suspendedSatellites" :key="suspended.id"><b> {{ suspended.id }}</b></span>. If you have any questions regarding this please check our Node Operators
                <a
                    class="info-area__disqualified-info__info__link"
                    href="https://forum.storj.io/c/sno-category"
                    rel="noopener noreferrer"
                    target="_blank"
                >
                    thread
                </a> on Storj forum.
            </p>
        </div>
        <p class="info-area__title">Bandwidth Utilization </p>
        <section>
            <div class="chart-container bandwidth-chart">
                <div class="chart-container__title-area">
                    <p class="chart-container__title-area__title">Bandwidth Used This Month</p>
                    <div class="chart-container__title-area__buttons-area">
                        <button
                            name="Show Egress Chart"
                            class="chart-container__title-area__chart-choice-item"
                            type="button"
                            :class="{ 'egress-chart-shown': isEgressChartShown }"
                            @click.stop="toggleEgressChartShowing"
                        >
                            Egress
                        </button>
                        <button
                            name="Show Ingress Chart"
                            class="chart-container__title-area__chart-choice-item"
                            type="button"
                            :class="{ 'ingress-chart-shown': isIngressChartShown }"
                            @click.stop="toggleIngressChartShowing"
                        >
                            Ingress
                        </button>
                    </div>
                </div>
                <p v-if="isBandwidthChartShown" class="chart-container__amount"><b>{{ bandwidthSummary }}</b></p>
                <p v-if="isEgressChartShown" class="chart-container__amount"><b>{{ egressSummary }}</b></p>
                <p v-if="isIngressChartShown" class="chart-container__amount"><b>{{ ingressSummary }}</b></p>
                <div ref="chart">
                    <BandwidthChart v-if="isBandwidthChartShown" :height="240" :width="chartWidth" :is-dark-mode="isDarkMode" />
                    <EgressChart v-if="isEgressChartShown" :height="240" :width="chartWidth" :is-dark-mode="isDarkMode" />
                    <IngressChart v-if="isIngressChartShown" :height="240" :width="chartWidth" :is-dark-mode="isDarkMode" />
                </div>
            </div>
        </section>
        <p class="info-area__title">Disk Utilization & Remaining</p>
        <section class="info-area__chart-area">
            <section class="chart-container">
                <div class="chart-container__title-area disk-space-title">
                    <p class="chart-container__title-area__title">Average Disk Space Used This Month</p>
                </div>
                <p class="chart-container__amount disk-space-amount"><b>{{ averageUsageBytes }}</b></p>
                <div ref="diskSpaceChart">
                    <DiskSpaceChart :height="240" :width="diskSpaceChartWidth" :is-dark-mode="isDarkMode" />
                </div>
            </section>
            <section>
                <DiskStatChart />
            </section>
        </section>
        <div>
            <p class="info-area__title">Suspension & Audit</p>
            <div v-if="selectedSatellite.id" class="info-area__checks-area">
                <ChecksArea
                    label="Suspension Score"
                    :amount="audits.suspensionScore.label"
                    info-text="This score shows how close your node is to getting suspended on a satellite. A score of 96% or below will result in suspension. If your node stays suspended for more than one week you will be disqualified from this satellite, so please correct the errors that lead to suspension asap."
                />
                <ChecksArea
                    label="Audit Score"
                    :amount="audits.auditScore.label"
                    info-text="Percentage of successful pings/communication between the node & satellite."
                />
                <ChecksArea
                    label="Online Score"
                    :amount="audits.onlineScore.label"
                    info-text="Online checks occur to make sure your node is still online. This is the percentage of online checks you’ve passed."
                />
            </div>
            <AllSatellitesAuditsArea v-else />
        </div>
        <div class="info-area__payout-header">
            <p class="info-area__title">Payout</p>
            <router-link :to="PAYOUT_PATH" class="info-area__payout-header__link">
                <p class="info-area__payout-header__link__text">Payout Information</p>
                <BlueArrowRight />
            </router-link>
        </div>
        <WalletArea
            label="Wallet Address"
            :wallet-address="nodeInfo.wallet"
            :wallet-features="nodeInfo.walletFeatures"
        />
        <TotalPayoutArea class="info-area__total-area" />
    </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue';

import { RouteConfig } from '@/app/router';
import { Size } from '@/private/memory/size';
import { Node, SatelliteInfo, SatelliteScores } from '@/storagenode/sno/sno';
import { useAppStore } from '@/app/store/modules/appStore';
import { useNodeStore } from '@/app/store/modules/nodeStore';

import AllSatellitesAuditsArea from '@/app/components/AllSatellitesAuditsArea.vue';
import BandwidthChart from '@/app/components/BandwidthChart.vue';
import ChecksArea from '@/app/components/ChecksArea.vue';
import DiskSpaceChart from '@/app/components/DiskSpaceChart.vue';
import DiskStatChart from '@/app/components/DiskStatChart.vue';
import EgressChart from '@/app/components/EgressChart.vue';
import IngressChart from '@/app/components/IngressChart.vue';
import SatelliteSelection from '@/app/components/SatelliteSelection.vue';
import TotalPayoutArea from '@/app/components/TotalPayoutArea.vue';
import WalletArea from '@/app/components/WalletArea.vue';

import LargeSuspensionIcon from '@/../static/images/largeSuspend.svg';
import LargeDisqualificationIcon from '@/../static/images/largeDisqualify.svg';
import BlueArrowRight from '@/../static/images/BlueArrowRight.svg';

const appStore = useAppStore();
const nodeStore = useNodeStore();

const PAYOUT_PATH = ref<string>(RouteConfig.Payout.path);
const chartWidth = ref<number>(0);
const diskSpaceChartWidth = ref<number>(0);

const chart = ref<HTMLElement>();
const diskSpaceChart = ref<HTMLElement>();

const isDarkMode = computed<boolean>(() => {
    return appStore.state.isDarkMode;
});

const isBandwidthChartShown = computed<boolean>(() => {
    return appStore.state.isBandwidthChartShown;
});

const isIngressChartShown = computed<boolean>(() => {
    return appStore.state.isIngressChartShown;
});

const isEgressChartShown = computed<boolean>(() => {
    return appStore.state.isEgressChartShown;
});

const nodeInfo = computed<Node>(() => {
    return nodeStore.state.info;
});

const bandwidthSummary = computed<string>(() => {
    return Size.toBase10String(nodeStore.state.bandwidthSummary);
});

const egressSummary = computed<string>(() => {
    return Size.toBase10String(nodeStore.state.egressSummary);
});

const ingressSummary = computed<string>(() => {
    return Size.toBase10String(nodeStore.state.ingressSummary);
});

const averageUsageBytes = computed<string>(() => {
    return Size.toBase10String(nodeStore.state.averageUsageBytes);
});

const audits = computed<SatelliteScores>(() => {
    return nodeStore.state.audits as SatelliteScores;
});

const selectedSatellite = computed<SatelliteInfo>(() => {
    return nodeStore.state.selectedSatellite;
});

const disqualifiedSatellites = computed<SatelliteInfo[]>(() => {
    return nodeStore.state.disqualifiedSatellites;
});

const isDisqualifiedInfoShown = computed<boolean>(() => {
    return !!(selectedSatellite.value.id && selectedSatellite.value.disqualified);
});

const getDisqualificationDate = computed<string>(() => {
    if (selectedSatellite.value.disqualified) {
        return selectedSatellite.value.disqualified.toUTCString();
    }

    return '';
});

const doDisqualifiedSatellitesExist = computed<boolean>(() => {
    return disqualifiedSatellites.value.length > 0;
});

const suspendedSatellites = computed<SatelliteInfo[]>(() => {
    return nodeStore.state.suspendedSatellites;
});

const isSuspendedInfoShown = computed<boolean>(() => {
    return !!(selectedSatellite.value.id && selectedSatellite.value.suspended);
});

const getSuspensionDate = computed<string>(() => {
    if (selectedSatellite.value.suspended) {
        return selectedSatellite.value.suspended.toUTCString();
    }

    return '';
});

const doSuspendedSatellitesExist = computed<boolean>(() => {
    return suspendedSatellites.value.length > 0;
});

function recalculateChartDimensions(): void {
    chartWidth.value = chart.value ? chart.value.clientWidth : 0;
    diskSpaceChartWidth.value = diskSpaceChart.value ? diskSpaceChart.value.clientWidth : 0;
}

function toggleEgressChartShowing(): void {
    if (isBandwidthChartShown.value || isIngressChartShown.value) {
        appStore.toggleEgressChart();

        return;
    }

    appStore.closeAdditionalCharts();
}

function toggleIngressChartShowing(): void {
    if (isBandwidthChartShown.value || isEgressChartShown.value) {
        appStore.toggleIngressChart();

        return;
    }

    appStore.closeAdditionalCharts();
}

onMounted(() => {
    window.addEventListener('resize', recalculateChartDimensions);
    recalculateChartDimensions();
});

onBeforeUnmount(() => {
    window.removeEventListener('resize', recalculateChartDimensions);
});
</script>

<style scoped lang="scss">
    p {
        margin-block: 0;
    }

    .info-area {
        width: 100%;
        padding: 0 0 30px;

        &__disqualified-info {
            display: flex;
            align-items: center;
            justify-content: space-between;
            padding: 20px 27px 20px 25px;
            background-color: var(--block-background-color);
            border-radius: 12px;
            width: calc(100% - 52px);
            margin-top: 17px;
            color: var(--regular-text-color);

            &__image {
                min-height: 35px;
                min-width: 38px;
                margin-right: 17px;
            }

            &__info {
                font-size: 14px;
                line-height: 21px;

                &__link {
                    color: var(--navigation-link-color);
                }
            }
        }

        &__suspended-info {
            display: flex;
            align-items: center;
            justify-content: space-between;
            padding: 20px 27px 20px 25px;
            background-color: #fcf8e3;
            border-radius: 12px;
            width: calc(100% - 52px);
            margin-top: 17px;

            &__image {
                min-height: 35px;
                min-width: 38px;
                margin-right: 17px;
            }

            &__info {
                font-size: 14px;
                line-height: 21px;

                &__link {
                    color: var(--navigation-link-color);
                }
            }
        }

        &__title {
            font-size: 18px;
            line-height: 57px;
            color: var(--regular-text-color);
            user-select: none;
        }

        &__blurred-checks {
            display: flex;
            align-items: center;
            justify-content: center;
            width: 100%;
            height: 224px;
            background-image: var(--blurred-image-path);
            background-size: contain;
            margin: 35px 0;

            &__title {
                font-family: 'font_bold', sans-serif;
                font-size: 22px;
                line-height: 49px;
                color: var(--regular-text-color);
                user-select: none;
            }
        }

        &__payout-header {
            display: flex;
            flex-direction: row;
            align-items: center;
            justify-content: space-between;

            &__link {
                display: flex;
                flex-direction: row;
                align-items: center;
                justify-content: flex-end;
                text-decoration: none;

                &__text {
                    font-size: 16px;
                    line-height: 22px;
                    color: var(--navigation-link-color);
                    margin-right: 9px;
                }
            }
        }

        &__chart-area,
        &__checks-area {
            display: flex;
            flex-direction: row;
            justify-content: space-between;
            width: 100%;
        }

        &__bar-info {
            width: 339px;
        }

        &__estimation-area {
            margin-top: 11px;
        }

        &__total-area {
            margin-top: 20px;
        }
    }

    .chart-container {
        width: 339px;
        height: 336px;
        background-color: var(--block-background-color);
        border: 1px solid var(--block-border-color);
        border-radius: 11px;
        padding: 32px 30px;
        margin-bottom: 13px;
        position: relative;

        &__title-area {
            display: flex;
            align-items: center;
            justify-content: space-between;

            &__buttons-area {
                display: flex;
                flex-direction: row;
                align-items: flex-end;
            }

            &__title {
                font-size: 14px;
                color: var(--regular-text-color);
                user-select: none;
            }

            &__chart-choice-item {
                padding: 5px 12px;
                background-color: var(--chart-selection-button-background-color);
                border-radius: 47px;
                font-size: 12px;
                color: #9daed2;
                max-height: 25px;
                cursor: pointer;
                user-select: none;
                margin-left: 9px;
            }
        }

        &__amount {
            font-family: 'font_bold', sans-serif;
            font-size: 32px;
            line-height: 57px;
            color: var(--regular-text-color);
        }
    }

    .egress-chart-shown {
        background-color: var(--egress-button-background-color);
        color: var(--egress-button-font-color);
    }

    .ingress-chart-shown {
        background-color: var(--ingress-button-background-color);
        color: var(--ingress-button-font-color);
    }

    .disk-space-title,
    .disk-space-amount {
        margin-top: 5px;
    }

    .bandwidth-chart {
        width: calc(100% - 60px);
    }

    @media screen and (width <= 1000px) {

        .info-area {

            &__chart-area {
                flex-direction: column;
                justify-content: flex-start;
            }
        }

        .chart-container {
            width: calc(100% - 60px);
        }
    }

    @media screen and (width <= 780px) {

        .info-area {

            &__checks-area {
                flex-direction: column;

                .checks-area-container {
                    width: calc(100% - 60px) !important;
                }
            }

            &__blurred-checks {

                &__title {
                    text-align: center;
                }
            }
        }
    }

    @media screen and (width <= 400px) {

        .chart-container {
            width: calc(100% - 36px);
            padding: 24px 18px;
        }
    }
</style>
