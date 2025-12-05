// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

<script lang="ts">
import { Component, Prop, Vue, Watch } from 'vue-property-decorator';

import VChart from '@/app/components/common/VChart.vue';

// @vue/component
@Component({
    components: {
        VChart,
    },
})
export default class BaseChart extends Vue {
    @Prop({ default: 0 })
    public width: number;
    @Prop({ default: 0 })
    public height: number;
    @Prop({ default: false })
    public isDarkMode: boolean;

    public chartWidth = 0;
    public chartHeight = 0;
    /**
     * Used for chart re rendering.
     */
    public chartKey = 0;

    public $refs: {
        chartContainer: HTMLElement;
    };

    @Watch('width')
    @Watch('isDarkMode')
    public rebuildChart(): void {
        this.chartHeight = this.height;
        this.chartWidth = this.width;
        this.chartKey += 1;
    }
    /**
     * Rebuilds chart after type switch.
     */
    public mounted(): void {
        this.rebuildChart();
    }
}
</script>

<style scoped lang="scss">
    .chart {
        width: 100%;
        height: 100%;

        &__data-dimension {
            font-size: 13px;
            color: var(--v-text-base);
            margin: 0 0 5px 31px !important;
            font-family: 'font_medium', sans-serif;
        }
    }

    @media screen and (max-width: 400px) {

        .chart-container {
            width: calc(100% - 36px);
            padding: 24px 18px;
        }
    }
</style>
