// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="countdown-container">
        <div v-if="isExpired">{{date}}</div>
        <div class="row" v-else>
            <p>Expires in </p>
            <p class="digit margin">{{ hours | leadingZero }}</p>
            <p>:</p>
            <p class="digit">{{ minutes | leadingZero }}</p>
            <p>:</p>
            <p class="digit">{{ seconds | leadingZero }}</p>
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Prop, Vue } from 'vue-property-decorator';

import { BillingHistoryItemType } from '@/types/payments';

@Component
export default class BillingHistoryDate extends Vue {
    @Prop({default: () => new Date()})
    private readonly expiration: Date;
    @Prop({default: () => new Date()})
    private readonly start: Date;
    @Prop({default: 0})
    private readonly type: BillingHistoryItemType;

    private readonly expirationTimeInSeconds: number;
    private nowInSeconds = Math.trunc(new Date().getTime() / 1000);
    private intervalID: number;

    public isExpired: boolean;

    public constructor() {
        super();

        this.expirationTimeInSeconds = Math.trunc(new Date(this.expiration).getTime() / 1000);
        this.isExpired = (this.expirationTimeInSeconds - this.nowInSeconds) < 0;

        this.ready();
    }

    public get date(): string {
        if (this.type === BillingHistoryItemType.Invoice) {
            return `${this.start.toLocaleDateString()} - ${this.expiration.toLocaleDateString()}`;
        }

        return this.start.toLocaleDateString();
    }

    public get seconds(): number {
        return (this.expirationTimeInSeconds - this.nowInSeconds) % 60;
    }

    public get minutes(): number {
        return Math.trunc((this.expirationTimeInSeconds - this.nowInSeconds) / 60) % 60;
    }

    public get hours(): number {
        return Math.trunc((this.expirationTimeInSeconds - this.nowInSeconds) / 3600) % 24;
    }

    private ready(): void {
        this.intervalID = window.setInterval(() => {
            if ((this.expirationTimeInSeconds - this.nowInSeconds) < 0) {
                this.isExpired = true;
                clearInterval(this.intervalID);

                return;
            }

            this.nowInSeconds = Math.trunc(new Date().getTime() / 1000);
        }, 1000);
    }
}
</script>

<style scoped lang="scss">
    .digit {
        font-family: 'font_bold', sans-serif;
    }

    .margin {
        margin-left: 5px;
    }

    .row {
        display: flex;
        align-items: center;
        justify-content: flex-start;
    }
</style>
