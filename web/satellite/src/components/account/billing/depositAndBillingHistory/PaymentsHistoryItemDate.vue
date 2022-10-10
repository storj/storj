// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="countdown-container">
        <div v-if="isExpired">{{ date }}</div>
        <div v-else class="row">
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

import { PaymentsHistoryItemStatus, PaymentsHistoryItemType } from '@/types/payments';

// @vue/component
@Component({
    filters: {
        leadingZero(value: number): string {
            if (value <= 9) {
                return `0${value}`;
            }
            return `${value}`;
        },
    },
})
export default class PaymentsHistoryItemDate extends Vue {
    /**
     * expiration date.
     */
    @Prop({ default: () => new Date() })
    private readonly expiration: Date;
    /**
     * creation date.
     */
    @Prop({ default: () => new Date() })
    private readonly start: Date;
    @Prop({ default: 0 })
    private readonly type: PaymentsHistoryItemType;
    @Prop({ default: '' })
    private readonly status: PaymentsHistoryItemStatus;

    private expirationTimeInSeconds: number;
    private nowInSeconds = Math.trunc(new Date().getTime() / 1000);
    private intervalID: ReturnType<typeof setInterval>;

    /**
     * indicates if billing item is expired.
     */
    public isExpired: boolean;

    public created() {
        this.expirationTimeInSeconds = Math.trunc(new Date(this.expiration).getTime() / 1000);
        this.isExpired = (this.expirationTimeInSeconds - this.nowInSeconds) < 0;

        if (this.type === PaymentsHistoryItemType.Transaction) {
            this.isExpired = this.isTransactionCompleted;
        }

        this.ready();
    }

    /**
     * String representation of creation date.
     */
    public get date(): string {
        return this.start.toLocaleString('default', { month: 'long', day: '2-digit', year: 'numeric' });
    }

    /**
     * Seconds count for expiration timer.
     */
    public get seconds(): number {
        return (this.expirationTimeInSeconds - this.nowInSeconds) % 60;
    }

    /**
     * Minutes count for expiration timer.
     */
    public get minutes(): number {
        return Math.trunc((this.expirationTimeInSeconds - this.nowInSeconds) / 60) % 60;
    }

    /**
     * Hours count for expiration timer.
     */
    public get hours(): number {
        return Math.trunc((this.expirationTimeInSeconds - this.nowInSeconds) / 3600) % 24;
    }

    /**
     * Indicates if transaction status is completed, paid or cancelled.
     */
    private get isTransactionCompleted(): boolean {
        return this.status !== PaymentsHistoryItemStatus.Pending;
    }

    /**
     * Starts expiration timer if item is not expired.
     */
    private ready(): void {
        this.intervalID = setInterval(() => {
            if ((this.expirationTimeInSeconds - this.nowInSeconds) < 0 || this.isTransactionCompleted) {
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
