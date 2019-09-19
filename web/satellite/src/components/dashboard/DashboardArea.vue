// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div>
        <EmptyState
            main-title="Create your first new project"
            :image-source="emptyImage" />
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import EmptyState from '@/components/common/EmptyStateArea.vue';

import { EMPTY_STATE_IMAGES } from '@/utils/constants/emptyStatesImages';

@Component({
    components: {
        EmptyState,
    },
})
export default class DashboardArea extends Vue {
    public emptyImage: string = EMPTY_STATE_IMAGES.PROJECT;
}
</script>
