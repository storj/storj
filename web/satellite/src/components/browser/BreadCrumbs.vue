<script lang="ts"> /* eslint-disable */ </script>

// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

<style scoped lang="css">
/* stylelint-disable */

.path {
	font-size: 18px;
	color: #232b34;
	font-weight: bold;
	cursor: pointer;
}
.path:hover {
	color: #376fff;
}
.path-buckets {
	font-size: 18px;
	color: #232b34;
}
</style>

<template>
	<div class="my-3">
		<div class="d-inline">
			<p class="d-inline path-buckets">Buckets</p>
			<svg
				class="mx-3"
				width="6"
				height="11"
				viewBox="0 0 6 11"
				fill="none"
				xmlns="http://www.w3.org/2000/svg"
			>
				<path
					opacity="0.5"
					fill-rule="evenodd"
					clip-rule="evenodd"
					d="M0.254166 0.280039C-0.0847221 0.653424 -0.0847221 1.2588 0.254166 1.63219L3.54555 5.25862L0.254166 8.88505C-0.0847225 9.25844 -0.0847225 9.86382 0.254166 10.2372C0.593054 10.6106 1.1425 10.6106 1.48139 10.2372L6 5.25862L1.48139 0.280039C1.1425 -0.0933463 0.593054 -0.0933463 0.254166 0.280039Z"
					fill="black"
				/>
			</svg>
		</div>

		<div v-for="(path, idx) in crumbs" class="d-inline" v-bind:key="idx">
			<router-link v-bind:to="link(idx)">
				<a class="path" href="javascript:null">{{ path }}</a>
			</router-link>

			<svg
				v-if="displayDivider(idx)"
				class="mx-3"
				width="6"
				height="11"
				viewBox="0 0 6 11"
				fill="none"
				xmlns="http://www.w3.org/2000/svg"
			>
				<path
					opacity="0.5"
					fill-rule="evenodd"
					clip-rule="evenodd"
					d="M0.254166 0.280039C-0.0847221 0.653424 -0.0847221 1.2588 0.254166 1.63219L3.54555 5.25862L0.254166 8.88505C-0.0847225 9.25844 -0.0847225 9.86382 0.254166 10.2372C0.593054 10.6106 1.1425 10.6106 1.48139 10.2372L6 5.25862L1.48139 0.280039C1.1425 -0.0933463 0.593054 -0.0933463 0.254166 0.280039Z"
					fill="black"
				/>
			</svg>
		</div>
	</div>
</template>

<script>
export default {
	name: "BreadCrumbs",
	computed: {
		bucketName() {
			return this.$store.state.files.bucket;
		},

		crumbs() {
			let path = this.$store.state.files.path.split("/");
			path =
				path.length > 1
					? [this.bucketName, ...path.slice(0, path.length - 1)]
					: [this.bucketName];
			return path;
		}
	},
	methods: {
		link(idx) {
			const crumbs = this.crumbs;
			let path = "";
			if (idx > 0) path = crumbs.slice(1, idx + 1).join("/") + "/";

			return this.$store.state.files.browserRoot + path;
		},
		displayDivider(idx) {
			const length = this.crumbs.length;
			return (idx !== 0 || length > 1) && idx !== length - 1;
		}
	}
};
</script>
