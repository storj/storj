// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.
<template>
	<div class="access-grants">
		<h2 class="access-grants__title">Access Grants</h2>
		<EmptyState />
	</div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import EmptyState from './EmptyState.vue';

@Component({
    components: {
        EmptyState,
    },
})

export default class AccessGrants extends Vue {
}
</script>
<style scoped lang="scss">
    .access-grants {
        position: relative;
        padding: 40px 30px 55px 30px;
        font-family: 'font_regular', sans-serif;

        &__title {
            font-family: 'font_bold', sans-serif;
            font-size: 32px;
            line-height: 39px;
            color: #263549;
            margin: 0;
        }
    }
</style>
