// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.
<template>
	<div class="access-grants">
		<h1 class="access-grants__title">Access Grants</h1>
		<div class="access-grants__modal">
			<h4 class="access-grants__modal__heading">Create Your First Access Grant</h4>
			<p class="access-grants__modal__subheading">Get started by creating an Access to interact with your Buckets</p>
			<p></p>
		</div>
	</div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

// import Background from '@/../static/images/accessGrants/access-grants-bg.svg';

// @Component({
//     components: {
//         Background,
//     },
// })

export default class CreateAccessUplinkStep extends Vue {
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

    &__modal {
        display: block;
        margin-left: auto;
        margin-right: auto;
        width: 660px;
        text-align: center;

        &__heading {
            font-family: 'font_bold', sans-serif;
            font-size: 28px;
            font-weight: 700;
            line-height: 16px;
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
