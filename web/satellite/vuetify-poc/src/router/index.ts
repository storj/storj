// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

// Composables
import { createRouter, createWebHistory } from 'vue-router'

const routes = [
  {
    path: '/vuetifypoc',
    redirect: { path: '/projects' }, // redirect
    component: () => import('@poc/layouts/default/Default.vue'),
    children: [
      {
        path: '/dashboard',
        name: 'Dashboard',
        component: () => import(/* webpackChunkName: "home" */ '@poc/views/Dashboard.vue'),
      },
      {
        path: '/buckets',
        name: 'Buckets',
        component: () => import(/* webpackChunkName: "Buckets" */ '@poc/views/Buckets.vue'),
      },
      {
        path: '/bucket',
        name: 'Bucket',
        component: () => import(/* webpackChunkName: "Buckets" */ '@poc/views/Bucket.vue'),
      },
      {
        path: '/access',
        name: 'Access',
        component: () => import(/* webpackChunkName: "Access" */ '@poc/views/Access.vue'),
      },
      {
        path: '/team',
        name: 'Team',
        component: () => import(/* webpackChunkName: "Team" */ '@poc/views/Team.vue'),
      },
    ],
  },
  {
    path: '/account',
    component: () => import('@poc/layouts/default/Account.vue'),
    children: [
      {
        path: '/billing',
        name: 'Billing',
        component: () => import(/* webpackChunkName: "Billing" */ '@poc/views/Billing.vue'),
      },
      {
        path: '/account-settings',
        name: 'Account Settings',
        component: () => import(/* webpackChunkName: "MyAccount" */ '@poc/views/AccountSettings.vue'),
      },
      {
        path: '/design-library',
        name: 'Design Library',
        component: () => import(/* webpackChunkName: "DesignLibrary" */ '@poc/views/DesignLibrary.vue'),
      },
    ],
  },
  {
    path: '/projects',
    component: () => import('@poc/layouts/default/AllProjects.vue'),
    children: [
      {
        path: '/projects',
        name: 'Projects',
        component: () => import(/* webpackChunkName: "Projects" */ '@poc/views/Projects.vue'),
      },
    ],
  },
]

const router = createRouter({
  history: createWebHistory(),
  routes,
})

export default router
