// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="pt-bar">
        <p class="pt-bar__info">
            You are currently using
            <VLoader
                v-if="isDataFetching"
                class="pt-bar__info__loader"
                is-white="true"
                width="20px"
                height="20px"
            />
            <b class="pt-bar__info__bold" v-else>{{storageUsed}}</b>
            storage out of the
            <b class="pt-bar__info__bold">150GB</b>
            included in the free account.
        </p>
        <p class="pt-bar__functional">
            Upload up to 75TB.
            <b class="pt-bar__info__bold upgrade" @click.stop="openAddPMModal">Upgrade now.</b>
        </p>
    </div>
</template>

<script lang="ts">
import { Component, Prop, Vue } from 'vue-property-decorator';

import VLoader from '@/components/common/VLoader.vue';

import { PAYMENTS_MUTATIONS } from '@/store/modules/payments';
import { PROJECTS_ACTIONS } from '@/store/modules/projects';
import { ProjectLimits } from '@/types/projects';
import { Size } from '@/utils/bytesSize';

@Component({
    components: {
        VLoader,
    },
})
export default class PaidTierBar extends Vue {
    @Prop({default: () => false})
    public readonly openAddPMModal: () => void;

    /**
     * Mounted lifecycle hook after initial render.
     * Fetches total limits.
     */
    public async mounted(): Promise<void> {
        try {
            await this.$store.dispatch(PROJECTS_ACTIONS.GET_TOTAL_LIMITS);
        } catch (error) {
            await this.$notify.error(error.message);
        }

        await this.$store.commit(PAYMENTS_MUTATIONS.TOGGLE_PAID_TIER_BANNER_TO_LOADED);
    }

    /**
     * Indicates if total limits data is fetching.
     */
    public get isDataFetching(): boolean {
        return this.$store.state.paymentsModule.isPaidTierBarLoading;
    }

    /**
     * Returns formatted string of used storage.
     */
    public get storageUsed(): string {
        if (this.totalLimits.storageUsed === 0) {
            return '0';
        }

        const used = new Size(this.totalLimits.storageUsed, 0);

        return `${used.formattedBytes}${used.label}`;
    }

    /**
     * Returns total limits from store.
     */
    private get totalLimits(): ProjectLimits {
        return this.$store.state.projectsModule.totalLimits;
    }
}
</script>

<style scoped lang="scss">
    .pt-bar {
        font-family: 'font_regular', sans-serif;
        display: flex;
        align-items: center;
        justify-content: space-between;
        background: #0047ff;
        font-size: 14px;
        line-height: 18px;
        color: #eee;
        padding: 5px 30px;

        &__info,
        &__functional {
            display: flex;
            align-items: center;
            white-space: nowrap;

            &__bold {
                margin: 0 5px;
                font-family: 'font_bold', sans-serif;
            }

            &__loader {
                margin: 0 5px;
            }
        }
    }

    .upgrade {
        cursor: pointer;
    }
</style>
