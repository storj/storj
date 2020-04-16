// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="title-area">
        <div class="title-area__node-id-container">
            <b class="title-area__node-id-container__title">Node ID:</b>
            <p class="title-area__node-id-container__id">{{ this.nodeId }}</p>
        </div>
        <h1 class="title-area__title">Your Storage Node Stats</h1>
        <div class="title-area__info-container">
            <div class="title-area__info-container__info-item">
                <p class="title-area__info-container__info-item__title">STATUS</p>
                <p v-if="online" class="title-area__info-container__info-item__content online-status">Online</p>
                <p v-else class="title-area__info-container__info-item__content offline-status">Offline</p>
            </div>
            <div class="title-area-divider"></div>
            <div class="title-area__info-container__info-item">
                <p class="title-area__info-container__info-item__title">UPTIME</p>
                <p class="title-area__info-container__info-item__content">{{ uptime }}</p>
            </div>
            <div class="title-area-divider"></div>
            <div class="title-area__info-container__info-item">
                <p class="title-area__info-container__info-item__title">LAST CONTACT</p>
                <p class="title-area__info-container__info-item__content">{{ lastPinged }} ago</p>
            </div>
            <div class="title-area-divider"></div>
            <VInfo
                v-if="info.isLastVersion"
                text="Running the minimal allowed version:"
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
            <div class="title-area-divider"></div>
            <div class="title-area__info-container__info-item">
                <p class="title-area__info-container__info-item__title">PERIOD</p>
                <p class="title-area__info-container__info-item__content">{{ currentMonth }}</p>
            </div>
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import VInfo from '@/app/components/VInfo.vue';

import { StatusOnline } from '@/app/store/modules/node';
import { Duration, millisecondsInSecond, minutesInHour, secondsInHour, secondsInMinute } from '@/app/utils/duration';

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

    public constructor(id: string, status: string, version: string, allowedVersion: string, wallet: string, isLastVersion: boolean) {
        this.id = id;
        this.status = status;
        this.version = this.toVersionString(version);
        this.allowedVersion = this.toVersionString(allowedVersion);
        this.wallet = wallet;
        this.isLastVersion = isLastVersion;
    }

    private toVersionString(version: string): string {
        return `v${version}`;
    }
}

@Component ({
    components: {
        VInfo,
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
            nodeInfo.isLastVersion);
    }

    public get online(): boolean {
        return this.$store.state.node.info.status === StatusOnline;
    }

    public get lastPinged(): string {
        return this.timePassed(this.$store.state.node.info.lastPinged);
    }

    public get uptime(): string {
        return this.timePassed(this.$store.state.node.info.startedAt);
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
    .title-area {
        font-family: 'font_regular', sans-serif;
        margin-bottom: 9px;

        &__node-id-container {
            color: var(--regular-text-color);
            height: 44px;
            padding: 14px;
            border: 1px solid #e8e8e8;
            border-radius: 12px;
            font-size: 14px;
            margin-right: 30px;
            display: none;

            &__title {
                min-width: 55px;
                margin-right: 5px;
            }

            &__id {
                font-size: 11px;
            }
        }

        &__title {
            font-family: 'font_bold', sans-serif;
            margin: 0 0 21px 0;
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
                    margin: 0 0 5px 0;
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

    /deep/ .info__message-box {
        background-image: var(--info-image-arrow-left-path);
        bottom: 100%;
        left: 220%;
        padding: 20px 20px 25px 20px;

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
                margin: 0 0 20px 0;
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
                justify-content: flex-start;

                &__info-item {
                    padding: 12px 8px;
                }
            }
        }

        .title-area-divider {
            display: none;
        }
    }
</style>
