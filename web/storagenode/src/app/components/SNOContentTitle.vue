// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="title-area">
        <div v-clipboard="nodeId" class="title-area__node-id-container">
            <b class="title-area__node-id-container__title">Node ID</b>
            <div class="title-area__node-id-container__right-area">
                <p class="title-area__node-id-container__id">{{ nodeId }}</p>
                <CopyIcon />
            </div>
        </div>
        <h1 class="title-area__title">Your Storage Node Stats</h1>
        <div class="title-area__info-container">
            <div class="title-area__info-container__info-item">
                <p class="title-area__info-container__info-item__title">STATUS</p>
                <p v-if="online" class="title-area__info-container__info-item__content online-status">Online</p>
                <p v-else class="title-area__info-container__info-item__content offline-status">Offline</p>
            </div>
            <div class="title-area-divider" />

            <VInfo
                v-if="info.quicStatus === quicStatusRefreshing"
                text="Testing connection to node via QUIC"
            >
                <div class="title-area__info-container__info-item">
                    <p class="title-area__info-container__info-item__title">QUIC</p>
                    <p class="title-area__info-container__info-item__content">{{ quicStatusRefreshing }}</p>
                </div>
            </VInfo>
            <VInfo
                v-if="info.quicStatus === quicStatusOk"
                :text="'QUIC is configured to use UDP port ' + info.configuredPort"
                :bold-text="'Last pinged ' + lastQuicPingedAt + ' ago'"
            >
                <div class="title-area__info-container__info-item">
                    <p class="title-area__info-container__info-item__title">QUIC</p>
                    <p class="title-area__info-container__info-item__content online-status">{{ quicStatusOk }}</p>
                </div>
            </VInfo>
            <VInfo
                v-if="info.quicStatus === quicStatusMisconfigured"
                :text="'QUIC is misconfigured. You must forward port ' + info.configuredPort + ' for both TCP and UDP to enable QUIC.'"
                bold-text="See https://docs.storj.io/node/dependencies/port-forwarding on how to do this"
            >
                <div class="title-area__info-container__info-item">
                    <p class="title-area__info-container__info-item__title">QUIC</p>
                    <p class="title-area__info-container__info-item__content offline-status">Misconfigured</p>
                </div>
            </VInfo>

            <div class="title-area-divider" />
            <div class="title-area__info-container__info-item">
                <p class="title-area__info-container__info-item__title">UPTIME</p>
                <p class="title-area__info-container__info-item__content">{{ uptime }}</p>
            </div>
            <div class="title-area-divider" />
            <div class="title-area__info-container__info-item">
                <p class="title-area__info-container__info-item__title">LAST CONTACT</p>
                <p class="title-area__info-container__info-item__content">{{ lastPinged }} ago</p>
            </div>
            <div class="title-area-divider" />
            <VInfo
                v-if="info.isLastVersion"
                text="Your node is up to date. Current minimum version:"
                :bold-text="info.allowedVersion"
            >
                <div class="title-area__info-container__info-item">
                    <p class="title-area__info-container__info-item__title">VERSION</p>
                    <p class="title-area__info-container__info-item__content">{{ info.version }}</p>
                </div>
            </VInfo>
            <VInfo
                v-if="!info.isLastVersion"
                text="Your node is outdated. Please update to:"
                :bold-text="info.allowedVersion"
            >
                <div class="title-area__info-container__info-item">
                    <p class="title-area__info-container__info-item__title">VERSION</p>
                    <p class="title-area__info-container__info-item__content">{{ info.version }}</p>
                </div>
            </VInfo>
            <div class="title-area-divider" />
            <div class="title-area__info-container__info-item">
                <p class="title-area__info-container__info-item__title">PERIOD</p>
                <p class="title-area__info-container__info-item__content">{{ currentMonth }}</p>
            </div>
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import { StatusOnline, QUIC_STATUS } from '@/app/store/modules/node';
import { Duration, millisecondsInSecond, minutesInHour, secondsInHour, secondsInMinute } from '@/app/utils/duration';

import VInfo from '@/app/components/VInfo.vue';

import CopyIcon from '@/../static/images/Copy.svg';

/**
 * NodeInfo class holds info for NodeInfo entity.
 */
class NodeInfo {
    public id: string;
    public status: string;
    public version: string;
    public allowedVersion: string;
    public wallet: string;
    public isLastVersion: boolean;
    public quicStatus: string;
    public configuredPort: string;

    public constructor(id: string, status: string, version: string, allowedVersion: string, wallet: string, isLastVersion: boolean, quicStatus: string, port: string) {
        this.id = id;
        this.status = status;
        this.version = this.toVersionString(version);
        this.allowedVersion = this.toVersionString(allowedVersion);
        this.wallet = wallet;
        this.isLastVersion = isLastVersion;
        this.quicStatus = quicStatus;
        this.configuredPort = port;
    }

    private toVersionString(version: string): string {
        return `v${version}`;
    }
}

// @vue/component
@Component ({
    components: {
        VInfo,
        CopyIcon,
    },
})
export default class SNOContentTitle extends Vue {
    private timeNow: Date = new Date();

    public mounted(): void {
        window.setInterval(() => {
            this.timeNow = new Date();
        }, 1000);
    }

    public get nodeId(): string {
        return this.$store.state.node.info.id;
    }

    public get info(): NodeInfo {
        const nodeInfo = this.$store.state.node.info;

        return new NodeInfo(nodeInfo.id, nodeInfo.status, nodeInfo.version, nodeInfo.allowedVersion, nodeInfo.wallet,
            nodeInfo.isLastVersion, nodeInfo.quicStatus, nodeInfo.configuredPort);
    }

    public get online(): boolean {
        return this.$store.state.node.info.status === StatusOnline;
    }

    public get quicStatusOk(): string {
        return QUIC_STATUS.StatusOk;
    }

    public get quicStatusRefreshing(): string {
        return QUIC_STATUS.StatusRefreshing;
    }

    public get quicStatusMisconfigured(): string {
        return QUIC_STATUS.StatusMisconfigured;
    }

    public get uptime(): string {
        return this.timePassed(this.$store.state.node.info.startedAt);
    }

    public get lastPinged(): string {
        return this.timePassed(this.$store.state.node.info.lastPinged);
    }

    public get lastQuicPingedAt(): string {
        return this.timePassed(this.$store.state.node.info.lastQuicPingedAt);
    }

    public get currentMonth(): string {
        const monthNames = ['January', 'February', 'March', 'April', 'May', 'June',
            'July', 'August', 'September', 'October', 'November', 'December',
        ];
        const date = new Date();

        return monthNames[date.getMonth()];
    }

    private timePassed(date: Date): string {
        const difference = Duration.difference(this.timeNow, date);

        if (Math.floor(difference / millisecondsInSecond) > secondsInHour) {
            const hours: string = Math.floor(difference / millisecondsInSecond / secondsInHour) + 'h';
            const minutes: string = Math.floor((difference / millisecondsInSecond % secondsInHour) / minutesInHour) + 'm';

            return `${hours} ${minutes}`;
        }

        return `${Math.floor(difference / millisecondsInSecond / secondsInMinute)}m`;
    }
}
</script>

<style scoped lang="scss">
    .svg ::v-deep path {
        fill: var(--node-id-copy-icon-color);
    }

    .title-area {
        font-family: 'font_regular', sans-serif;
        margin-bottom: 9px;

        &__node-id-container {
            color: var(--regular-text-color);
            height: 44px;
            padding: 14px;
            border: 1px solid var(--node-id-border-color);
            border-radius: 12px;
            font-size: 14px;
            margin-right: 30px;
            display: none;
            cursor: pointer;

            &__title {
                font-family: 'font_bold', sans-serif;
                min-width: 55px;
                margin-right: 5px;
            }

            &__id {
                margin-right: 20px;
                font-size: 11px;
            }

            &__right-area {
                display: flex;
                align-items: center;
                justify-content: flex-end;
            }

            &:hover {
                border-color: var(--node-id-border-hover-color);
                color: var(--node-id-hover-text-color);

                .svg ::v-deep path {
                    fill: var(--node-id-border-hover-color) !important;
                }
            }
        }

        &__title {
            font-family: 'font_bold', sans-serif;
            margin: 0 0 21px;
            font-size: 32px;
            line-height: 57px;
            color: var(--regular-text-color);
        }

        &__info-container {
            display: flex;
            justify-content: space-between;
            align-items: center;
            flex-wrap: wrap;

            &__info-item {
                padding: 15px 0;

                &__title {
                    font-size: 12px;
                    line-height: 20px;
                    color: #9ca5b6;
                    margin: 0 0 5px;
                }

                &__content {
                    font-size: 18px;
                    line-height: 20px;
                    font-family: 'font_medium', sans-serif;
                    color: var(--regular-text-color);
                    margin: 0;
                }
            }
        }
    }

    .title-area-divider {
        width: 1px;
        height: 22px;
        background-color: #dbdfe5;
    }

    .online-status {
        color: #519e62;
    }

    .offline-status {
        color: #ce0000;
    }

    ::v-deep .info__message-box {
        background-image: var(--info-image-arrow-left-path);
        bottom: 100%;
        left: 220%;
        padding: 20px 20px 25px;

        &__text {
            align-items: flex-start;

            &__regular-text {
                margin-bottom: 5px;
            }
        }
    }

    @media screen and (max-width: 780px) {

        .title-area {

            &__node-id-container {
                display: flex;
                align-items: center;
                justify-content: space-between;
                margin: 0 0 20px;
                height: auto;

                &__id {
                    word-break: break-all;
                }
            }
        }
    }

    @media screen and (max-width: 600px) {

        .title-area {

            &__title {
                font-size: 20px;
            }

            &__info-container {

                &__info-item {
                    padding: 12px 8px;
                }
            }
        }

        .title-area-divider {
            display: none;
        }
    }

    @media screen and (max-width: 600px) {

        .title-area {

            &__info-container {
                justify-content: flex-start;
            }
        }
    }
</style>
