// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div>
        <EmptyState
            mainTitle="Create your first new project"
            :imageSource="emptyImage" />
    </div>
</template>

<script lang="ts">
    import { Component, Vue } from 'vue-property-decorator';
    import EmptyState from '@/components/common/EmptyStateArea.vue';
    import { EMPTY_STATE_IMAGES } from '@/utils/constants/emptyStatesImages';

    @Component({
        components: {
            EmptyState,
        }
    })
    export default class DashboardArea extends Vue {
        public emptyImage: string = EMPTY_STATE_IMAGES.PROJECT;
    }
</script>
