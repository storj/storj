// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <!-- Support Icon -->
    <svg width="18" height="18" viewBox="0 0 20 20" fill="none" xmlns="http://www.w3.org/2000/svg">
      <path :fill="color" d="M3 1.79302C3 1.21459 3.59804 0.834834 4.1184 1.07137L4.14619 1.08464L16.8182 7.42064C17.3915 7.70729 17.4017 8.51596 16.8489 8.82126L16.8182 8.83741L4.14619 15.1734C4.10071 15.1962 4.05453 15.2141 4.00811 15.2275L4.008 18.496C4.008 18.7744 3.78235 19 3.504 19C3.23493 19 3.0151 18.7891 3.00075 18.5237L3 18.496V1.79302ZM4.008 2.14235V14.1155L15.9812 8.12901L4.008 2.14235Z"/>
    </svg>

</template>

<script>
  export default {
    props: {
      color: {
        type: String,
        default: 'currentColor'
      }
    }
  }
</script>