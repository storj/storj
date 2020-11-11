
// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.
<template>
	<div class="empty-state">
		<div class="empty-state__modal">
			<Key />
			<h4 class="empty-state__modal__heading">Create Your First Access Grant</h4>
			<p class="empty-state__modal__subheading">Get started by creating an Access to interact with your Buckets</p>
			<VButton
				label="Create Access Grant +"
				width="199px"
				height="44px"
				v-bind:style="{ margin: `0 auto`, marginTop: `25px`}"
			/>
		</div>
	</div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import VButton from '@/components/common/VButton.vue';

import Key from '@/../static/images/accessGrants/key.svg';

// import Background from '@/../static/images/accessGrants/access-grants-bg.svg';

@Component({
    components: {
        Key,
        VButton,
    },
})

export default class EmptyState extends Vue {
}
</script>

<style scoped lang="scss">
.empty-state {
    background-image: url('../../../static/images/accessGrants/access-grants-bg.png');

    &__modal {
        display: block;
        margin-left: auto;
        margin-right: auto;
        width: 660px;
        text-align: center;
        background: #fff;
        padding: 100px 30px;
        margin-top: 110px;

        &__heading {
            font-family: 'font_bold', sans-serif;
            font-size: 28px;
            font-weight: 700;
            line-height: 16px;
            margin-bottom: 30px;
        }

        &__subheading {
            font-family: 'font_regular', sans-serif;
            font-size: 16px;
            font-weight: 400;
            line-height: 21px;
        }
    }
}
</style>
