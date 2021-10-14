<script lang="ts"> /* eslint-disable */ </script>

// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

<style scoped>
/* stylelint-disable */

th {
	user-select: none;
	-moz-user-select: none;
	-khtml-user-select: none;
	-webkit-user-select: none;
	-o-user-select: none;
}

.arrow {
	cursor: pointer;
	color: #768394;
	position: absolute;
}

a {
	cursor: pointer;
}

.table-heading {
	cursor: pointer;
	color: #768394;
}
</style>

<template>
	<thead>
		<tr>
			<th
				v-on:mouseover="mouseOverName"
				v-on:mouseleave="mouseLeave"
				v-on:click="sortByName"
				class="table-heading"
				scope="col"
			>
				Name
				<span v-if="showNameArrow">
					<a class="arrow" v-if="nameDesc">
						<svg
							xmlns="http://www.w3.org/2000/svg"
							width="20"
							height="20"
							fill="currentColor"
							class="bi bi-arrow-down-short down-arrow"
							viewBox="0 0 16 16"
						>
							<path
								fill-rule="evenodd"
								d="M8 4a.5.5 0 0 1 .5.5v5.793l2.146-2.147a.5.5 0 0 1 .708.708l-3 3a.5.5 0 0 1-.708 0l-3-3a.5.5 0 1 1 .708-.708L7.5 10.293V4.5A.5.5 0 0 1 8 4z"
							/>
						</svg>
					</a>
					<a class="arrow" v-else>
						<svg
							xmlns="http://www.w3.org/2000/svg"
							width="20"
							height="20"
							fill="currentColor"
							class="bi bi-arrow-up-short"
							viewBox="0 0 16 16"
						>
							<path
								fill-rule="evenodd"
								d="M8 12a.5.5 0 0 0 .5-.5V5.707l2.146 2.147a.5.5 0 0 0 .708-.708l-3-3a.5.5 0 0 0-.708 0l-3 3a.5.5 0 1 0 .708.708L7.5 5.707V11.5a.5.5 0 0 0 .5.5z"
							/>
						</svg>
					</a>
				</span>
			</th>
			<th
				v-on:mouseover="mouseOverSize"
				v-on:mouseleave="mouseLeave"
				v-on:click="sortBySize"
				class="table-heading"
				scope="col"
			>
				Size
				<span v-if="showSizeArrow">
					<a class="arrow" v-if="sizeDesc">
						<svg
							xmlns="http://www.w3.org/2000/svg"
							width="20"
							height="20"
							fill="currentColor"
							class="bi bi-arrow-down-short down-arrow"
							viewBox="0 0 16 16"
						>
							<path
								fill-rule="evenodd"
								d="M8 4a.5.5 0 0 1 .5.5v5.793l2.146-2.147a.5.5 0 0 1 .708.708l-3 3a.5.5 0 0 1-.708 0l-3-3a.5.5 0 1 1 .708-.708L7.5 10.293V4.5A.5.5 0 0 1 8 4z"
							/>
						</svg>
					</a>
					<a class="arrow" v-else>
						<svg
							xmlns="http://www.w3.org/2000/svg"
							width="20"
							height="20"
							fill="currentColor"
							class="bi bi-arrow-up-short"
							viewBox="0 0 16 16"
						>
							<path
								fill-rule="evenodd"
								d="M8 12a.5.5 0 0 0 .5-.5V5.707l2.146 2.147a.5.5 0 0 0 .708-.708l-3-3a.5.5 0 0 0-.708 0l-3 3a.5.5 0 1 0 .708.708L7.5 5.707V11.5a.5.5 0 0 0 .5.5z"
							/>
						</svg>
					</a>
				</span>
			</th>
			<th
				v-on:mouseover="mouseOverDate"
				v-on:mouseleave="mouseLeave"
				v-on:click="sortByDate"
				class="table-heading"
				scope="col"
			>
				Upload Date
				<span v-if="showDateArrow">
					<a class="arrow" v-if="dateDesc">
						<svg
							xmlns="http://www.w3.org/2000/svg"
							width="20"
							height="20"
							fill="currentColor"
							class="bi bi-arrow-down-short down-arrow"
							viewBox="0 0 16 16"
						>
							<path
								fill-rule="evenodd"
								d="M8 4a.5.5 0 0 1 .5.5v5.793l2.146-2.147a.5.5 0 0 1 .708.708l-3 3a.5.5 0 0 1-.708 0l-3-3a.5.5 0 1 1 .708-.708L7.5 10.293V4.5A.5.5 0 0 1 8 4z"
							/>
						</svg>
					</a>
					<a class="arrow" v-else>
						<svg
							xmlns="http://www.w3.org/2000/svg"
							width="20"
							height="20"
							fill="currentColor"
							class="bi bi-arrow-up-short"
							viewBox="0 0 16 16"
						>
							<path
								fill-rule="evenodd"
								d="M8 12a.5.5 0 0 0 .5-.5V5.707l2.146 2.147a.5.5 0 0 0 .708-.708l-3-3a.5.5 0 0 0-.708 0l-3 3a.5.5 0 1 0 .708.708L7.5 5.707V11.5a.5.5 0 0 0 .5.5z"
							/>
						</svg>
					</a>
				</span>
			</th>
			<th class="table-heading" scope="col">
				<div class="dropleft">
					<a
						class="d-flex justify-content-end"
						id="header-delete"
						v-if="filesToDelete"
						v-on:click="deleteSelectedDropdown"
					>
						<svg
							xmlns="http://www.w3.org/2000/svg"
							width="24"
							height="24"
							fill="currentColor"
							class="bi bi-trash"
							viewBox="0 0 16 16"
						>
							<path
								d="M5.5 5.5A.5.5 0 0 1 6 6v6a.5.5 0 0 1-1 0V6a.5.5 0 0 1 .5-.5zm2.5 0a.5.5 0 0 1 .5.5v6a.5.5 0 0 1-1 0V6a.5.5 0 0 1 .5-.5zm3 .5a.5.5 0 0 0-1 0v6a.5.5 0 0 0 1 0V6z"
							/>
							<path
								fill-rule="evenodd"
								d="M14.5 3a1 1 0 0 1-1 1H13v9a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V4h-.5a1 1 0 0 1-1-1V2a1 1 0 0 1 1-1H6a1 1 0 0 1 1-1h2a1 1 0 0 1 1 1h3.5a1 1 0 0 1 1 1v1zM4.118 4L4 4.059V13a1 1 0 0 0 1 1h6a1 1 0 0 0 1-1V4.059L11.882 4H4.118zM2.5 3V2h11v1h-11z"
							/>
						</svg>
					</a>
					<div
						v-if="displayDropdown"
						class="dropdown-menu shadow show"
					>
						<div>
							<p class="deletion-confirmation px-5 pt-3">
								Are you sure?
							</p>
							<div class="d-flex">
								<button
									class="dropdown-item trash p-3 action"
									v-on:click="confirmDeleteSelection"
								>
									<svg
										xmlns="http://www.w3.org/2000/svg"
										width="16"
										height="16"
										fill="red"
										class="bi bi-trash"
										viewBox="0 0 16 16"
									>
										<path
											d="M5.5 5.5A.5.5 0 0 1 6 6v6a.5.5 0 0 1-1 0V6a.5.5 0 0 1 .5-.5zm2.5 0a.5.5 0 0 1 .5.5v6a.5.5 0 0 1-1 0V6a.5.5 0 0 1 .5-.5zm3 .5a.5.5 0 0 0-1 0v6a.5.5 0 0 0 1 0V6z"
										/>
										<path
											fill-rule="evenodd"
											d="M14.5 3a1 1 0 0 1-1 1H13v9a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V4h-.5a1 1 0 0 1-1-1V2a1 1 0 0 1 1-1H6a1 1 0 0 1 1-1h2a1 1 0 0 1 1 1h3.5a1 1 0 0 1 1 1v1zM4.118 4L4 4.059V13a1 1 0 0 0 1 1h6a1 1 0 0 0 1-1V4.059L11.882 4H4.118zM2.5 3V2h11v1h-11z"
										/>
									</svg>
									Yes
								</button>
								<button
									class="dropdown-item p-3 action"
									v-on:click="cancelDeleteSelection"
								>
									<svg
										width="2em"
										height="2em"
										viewBox="0 0 16 16"
										class="bi bi-x mr-1"
										fill="green"
										xmlns="http://www.w3.org/2000/svg"
									>
										<path
											fill-rule="evenodd"
											d="M4.646 4.646a.5.5 0 0 1 .708 0L8 7.293l2.646-2.647a.5.5 0 0 1 .708.708L8.707 8l2.647 2.646a.5.5 0 0 1-.708.708L8 8.707l-2.646 2.647a.5.5 0 0 1-.708-.708L7.293 8 4.646 5.354a.5.5 0 0 1 0-.708z"
										/>
									</svg>
									No
								</button>
							</div>
						</div>
					</div>
				</div>
			</th>
		</tr>
	</thead>
</template>

<script>
// Computed property creators

const isDesc = (heading) =>
	function () {
		return this.headingSorted === heading && this.orderBy === "desc";
	};

const showArrow = (heading) =>
	function () {
		return this.headingSorted === heading || this.hover === heading;
	};

const fromFilesStore = (prop) =>
	function () {
		return this.$store.state.files[prop];
	};

// Method creators

const mouseOver = (heading) =>
	function () {
		this.hover = heading;
	};

const sortBy = (heading) =>
	function () {
		this.$store.commit("files/sort", heading);
	};

export default {
	data: () => ({
		hover: null
	}),
	computed: {
		headingSorted: fromFilesStore("headingSorted"),
		orderBy: fromFilesStore("orderBy"),

		nameDesc: isDesc("name"),
		sizeDesc: isDesc("size"),
		dateDesc: isDesc("date"),

		showNameArrow: showArrow("name"),
		showSizeArrow: showArrow("size"),
		showDateArrow: showArrow("date"),

		filesToDelete() {
			return (
				!!this.$store.state.files.selectedAnchorFile ||
				!!(
					this.$store.state.files.unselectedAnchorFile &&
					(this.$store.state.files.selectedFiles.length > 0 ||
						this.$store.state.files.shiftSelectedFiles.length > 0)
				)
			);
		},

		displayDropdown() {
			return this.$store.state.files.openedDropdown === "FileBrowser";
		}
	},
	methods: {
		sortByName: sortBy("name"),
		sortBySize: sortBy("size"),
		sortByDate: sortBy("date"),

		mouseOverName: mouseOver("name"),
		mouseOverSize: mouseOver("size"),
		mouseOverDate: mouseOver("date"),

		mouseLeave() {
			this.hover = null;
		},

		deleteSelectedDropdown(event) {
			event.stopPropagation();
			this.$store.dispatch("files/openFileBrowserDropdown");
		},

		confirmDeleteSelection() {
			this.$store.dispatch("files/deleteSelected");
			this.$store.dispatch("files/closeDropdown");
		},

		cancelDeleteSelection() {
			this.$store.dispatch("files/closeDropdown");
		}
	}
};
</script>
