// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="title">
        <div class="title__name">
            <h1 class="title__name__title">Your Storage Node Stats</h1>
            <p class="title__name__info">Current period: <b>{{currentMonth}}</b></p>
        </div>
        <div class="title__info">
            <p class="title__info__status-title"><b>Node status</b></p>
            <p class="title__info__online-status">{{info.status}}</p>
            <VInfo
                v-if="online"
                bold-text="Last Pinged"
                extra-bold-text="Uptime"
                :green-text="lastPingedInMinutes"
                :extra-green-text="uptime"
                is-custom-position="true"
            >
                <div class="node-status-svg-container" @mouseenter.stop="refreshTime">
                    <OnlineIcon
                        class="check-if-online-svg"
                        alt="online status image"
                    />
                </div>
            </VInfo>
            <OfflineIcon
                class="check-if-offline-svg"
                v-if="!online"
                alt="offline status image"
            />
            <p class="title__info__version-title"><b>Node Version</b></p>
            <p class="title__info__version-value">{{version}}</p>
            <VInfo
                v-if="info.isLastVersion"
                text="Running the minimal allowed version:"
                bold-text="v0.0.0"
                is-custom-position="true"
            >
                <div class="version-svg-container">
                    <VersionIcon
                        class="version-svg"
                        alt="version status image"
                    />
                </div>
            </VInfo>
            <VInfo
                v-else
                text="Your node is outdated. Please update to:"
                bold-text="v.0.0.0"
                is-custom-position="true"
            >
                <div class="version-svg-container">
                    <VersionIcon
                        class="version-svg"
                        alt="version status image"
                    />
                </div>
            </VInfo>
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import VInfo from '@/app/components/VInfo.vue';

import OfflineIcon from '@/../static/images/offline.svg';
import OnlineIcon from '@/../static/images/online.svg';
import VersionIcon from '@/../static/images/version.svg';

import { StatusOnline } from '@/app/store/modules/node';
import { datesDiffInHoursAndMinutes } from '@/app/utils/date';

/**
 * NodeInfo class holds info for NodeInfo entity.
 */
class NodeInfo {
    public id: string;
    public status: string;
    public version: string;
    public wallet: string;
    public isLastVersion: boolean;

    public constructor(id: string, status: string, version: string, wallet: string, isLastVersion: boolean) {
        this.id = id;
        this.status = status;
        this.version = version;
        this.wallet = wallet;
        this.isLastVersion = isLastVersion;
    }
}

@Component ({
    components: {
        VInfo,
        OnlineIcon,
        OfflineIcon,
        VersionIcon,
    },
})
export default class SNOContentTitle extends Vue {
    private timeNow = new Date();

    public refreshTime(): void {
        this.timeNow = new Date();
    }

    public get info(): NodeInfo {
        return this.$store.state.node.info;
    }

    public get version(): string {
        const version = this.$store.state.node.info.version;

        return `v${version}`;
    }

    public get online(): boolean {
        return this.$store.state.node.info.status === StatusOnline;
    }

    public get lastPingedInMinutes(): string {
        const storedLastPinged: Date = this.$store.state.node.info.lastPinged;
        const shownLastPinged: string = datesDiffInHoursAndMinutes(this.timeNow, storedLastPinged);

        return `${shownLastPinged} ago`;
    }

    public get uptime(): string {
        const startedAt: Date = this.$store.state.node.info.startedAt;

        return datesDiffInHoursAndMinutes(this.timeNow, startedAt);
    }

    public get currentMonth(): string {
        const monthNames = ['January', 'February', 'March', 'April', 'May', 'June',
            'July', 'August', 'September', 'October', 'November', 'December'
        ];
        const date = new Date();

        return monthNames[date.getMonth()];
    }
}
</script>

<style scoped lang="scss">
    .title {
        display: flex;
        width: 100%;
        justify-content: space-between;
        align-items: center;
        color: #535F77;

        &__name {
            display: flex;
            align-items: center;

            &__title {
                margin: 0;
                font-size: 24px;
                line-height: 57px;
            }

            &__info {
                margin: 0 0 0 25px;
                font-size: 12px;
            }
        }

        &__info {
            display: flex;
            justify-content: space-between;
            align-items: center;
            font-size: 12px;
            position: relative;

            &__online-status {
                margin: 0 5px 0 5px;
            }

            &__version-title {
                margin-left: 35px;
            }

            &__version-value {
                margin: 0 5px 0 5px;
            }

            .version-svg-container,
            .node-status-svg-container {
                max-height: 18px;
                height: 18px;
            }
        }
    }

    .version-svg:hover,
    .check-if-online-svg:hover {
        cursor: pointer;
    }
</style>
