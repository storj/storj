// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

import { createRouter, createWebHistory } from 'vue-router';

import { useNodesStore } from '@/app/store/nodesStore';
import { NavigationLink } from '@/app/types/common';

import AddFirstNode from '@/app/views/AddFirstNode.vue';
import BandwidthPage from '@/app/views/bandwidth/BandwidthPage.vue';
import Dashboard from '@/app/views/Dashboard.vue';
import MyNodes from '@/app/views/myNodes/MyNodes.vue';
import PayoutsByNode from '@/app/views/payouts/PayoutsByNode.vue';
import PayoutsPage from '@/app/views/payouts/PayoutsPage.vue';
import PayoutsRoot from '@/app/views/payouts/PayoutsRoot.vue';
import WalletDetailsPage from '@/app/views/wallets/WalletDetailsPage.vue';
import WalletsPage from '@/app/views/wallets/WalletsPage.vue';
import WalletsRoot from '@/app/views/wallets/WalletsRoot.vue';
import WelcomeScreen from '@/app/views/WelcomeScreen.vue';

import MyNodesIcon from '@/../static/images/icons/navigation/nodes.svg';
import WalletsIcon from '@/../static/images/icons/navigation/wallets.svg';
import PayoutsIcon from '@/../static/images/icons/navigation/payouts.svg';
import TrafficIcon from '@/../static/images/icons/navigation/traffic.svg';

/**
 * Config contains configuration of all available routes for a Multinode Dashboard router.
 */
export class Config {
    public static Root = new NavigationLink('Root', '/');
    public static Welcome = new NavigationLink('Welcome', '/welcome');
    // nodes.
    public static AddFirstNode = new NavigationLink('AddFirstNode', '/add-first-node');
    public static MyNodes = new NavigationLink('My Nodes', '/my-nodes', MyNodesIcon);
    // payouts.
    public static PayoutsSummary = new NavigationLink('PayoutsSummary', 'summary');
    public static PayoutsByNode = new NavigationLink('PayoutsByNode', 'by-node/:id');
    public static Payouts = new NavigationLink('Payouts', '/payouts', PayoutsIcon);
    // bandwidth and disk.
    public static Bandwidth = new NavigationLink('Bandwidth & Disk', '/bandwidth', TrafficIcon);
    // wallets.
    public static WalletsSummary = new NavigationLink('WalletsSummary', 'summary');
    public static WalletDetails = new NavigationLink('WalletDetails', 'details/:address');
    public static Wallets = new NavigationLink('Wallets', '/wallets', WalletsIcon);
}

export const router = createRouter({
    history: createWebHistory(import.meta.env.PROD ? '' : '/'),
    routes: [
        {
            path: Config.Root.path,
            name: Config.Root.name,
            component: Dashboard,
            redirect: { name: Config.MyNodes.name },
            meta: {
                requiresAuth: true,
            },
            children: [
                {
                    path: Config.MyNodes.path,
                    name: Config.MyNodes.name,
                    component: MyNodes,
                },
                {
                    path: Config.Payouts.path,
                    name: Config.Payouts.name,
                    component: PayoutsRoot,
                    redirect: { name: Config.PayoutsSummary.name },
                    children: [
                        {
                            path: Config.PayoutsByNode.path,
                            name: Config.PayoutsByNode.name,
                            component: PayoutsByNode,
                        },
                        {
                            path: Config.PayoutsSummary.path,
                            name: Config.PayoutsSummary.name,
                            component: PayoutsPage,
                        },
                    ],
                },
                {
                    path: Config.Wallets.path,
                    name: Config.Wallets.name,
                    component: WalletsRoot,
                    redirect: { name: Config.WalletsSummary.name },
                    children: [
                        {
                            path: Config.WalletDetails.path,
                            name: Config.WalletDetails.name,
                            component: WalletDetailsPage,
                        },
                        {
                            path: Config.WalletsSummary.path,
                            name: Config.WalletsSummary.name,
                            component: WalletsPage,
                        },
                    ],
                },
                {
                    path: Config.Bandwidth.path,
                    name: Config.Bandwidth.name,
                    component: BandwidthPage,
                },
            ],
        },
        {
            path: Config.Welcome.path,
            name: Config.Welcome.name,
            component: WelcomeScreen,
        },
        {
            path: Config.AddFirstNode.path,
            name: Config.AddFirstNode.name,
            component: AddFirstNode,
        },
    ],
});

/**
 * List of allowed routes without any node added.
 */
const allowedRoutesNames = [Config.AddFirstNode.name, Config.Welcome.name];

/**
 * Checks if redirect to some of internal routes and no nodes added so far.
 * Redirect to Add first node screen if so.
 */
router.beforeEach(async(to, _from, next) => {
    const nodesStore = useNodesStore();
    if (nodesStore.state.nodes.length) {
        next();
        return;
    }

    if (!to.matched.some(record => allowedRoutesNames.includes(<string>record.name))) {
        await nodesStore.fetch();

        if (!nodesStore.state.nodes.length) {
            next(Config.AddFirstNode.path);
        } else {
            next();
        }
        return;
    }

    next();
});
