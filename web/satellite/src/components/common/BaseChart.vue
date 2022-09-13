// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

<script lang="ts">
import { Component, Prop, Vue, Watch } from 'vue-property-decorator';

// @vue/component
@Component
export default class BaseChart extends Vue {
    @Prop({ default: 0 })
    public readonly width: number;
    @Prop({ default: 0 })
    public readonly height: number;

    /**
     * Used for chart re rendering.
     */
    public chartKey = 0;

    /**
     * Triggers chart re rendering if chart width is being changed.
     */
    @Watch('width')
    public rebuildChart(): void {
        this.chartKey += 1;
    }
}
</script>
