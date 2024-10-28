// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

import Router, { RouterMode } from 'vue-router';
import { Component } from 'vue-router/types/router';

import { store } from '@/app/store';
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

/**
 * Metadata holds arbitrary information to routes like transition names, who can access the route, etc.
 */
class Metadata {
    public requiresAuth: boolean;
}

/**
 * Route holds all needed information to fill up router config.
 */
export class Route {
    public readonly path: string;
    public readonly name: string;
    public readonly component: Component;
    public children?: Route[];
    public readonly meta?: Metadata;
    public redirect?: Route;

    /**
     * default constructor.
     * @param path - route path.
     * @param name - name of the route, needed fot identifying route by name.
     * @param component - component mapped to route.
     * @param meta - arbitrary information to routes like transition names, who can access the route, etc.
     * @param redirect
     */
    public constructor(
        path: string,
        name: string,
        component: Component,
        meta: Metadata | undefined = undefined,
        redirect: Route | undefined = undefined,
    ) {
        this.path = path;
        this.name = name;
        this.component = component;
        this.meta = meta;
        this.redirect = redirect;
    }

    /**
     * Adds children routes to route.
     */
    public addChildren(children: Route[]): Route {
        this.children = children;

        return this;
    }

    /**
     * indicates if this route is a child route.
     */
    public isChild(): boolean {
        return this.path[0] !== '/';
    }

    /**
     * combines child route with its ancestor.
     * @param child
     */
    public with(child: Route): Route {
        if (!child.isChild()) {
            throw new Error('provided child root is not defined');
        }

        return new Route(`${this.path}/${child.path}`, child.name, child.component, child.meta, child.redirect);
    }
}

/**
 * Config contains configuration of all available routes for a Multinode Dashboard router.
 */
export class Config {
    public static Root: Route = new Route('/', 'Root', Dashboard, { requiresAuth: true });
    public static Welcome: Route = new Route('/welcome', 'Welcome', WelcomeScreen);
    // nodes.
    public static AddFirstNode: Route = new Route('/add-first-node', 'AddFirstNode', AddFirstNode);
    public static MyNodes: Route = new Route('/my-nodes', 'My Nodes', MyNodes);
    // payouts.
    public static PayoutsSummary: Route = new Route('summary', 'PayoutsSummary', PayoutsPage);
    public static PayoutsByNode: Route = new Route('by-node/:id', 'PayoutsByNode', PayoutsByNode);
    public static Payouts: Route = new Route('/payouts', 'Payouts', PayoutsRoot, undefined, Config.PayoutsSummary);
    // bandwidth and disk.
    public static Bandwidth: Route = new Route('/bandwidth', 'Bandwidth & Disk', BandwidthPage);
    // wallets.
    public static WalletsSummary: Route = new Route('summary', 'WalletsSummary', WalletsPage);
    public static WalletDetails: Route = new Route('details/:address', 'WalletDetails', WalletDetailsPage);
    public static Wallets: Route = new Route('/wallets', 'Wallets', WalletsRoot, undefined, Config.WalletsSummary);

    public static mode: RouterMode = 'history';
    public static routes: Route[] = [
        Config.Root.addChildren([
            Config.MyNodes,
            Config.Payouts.addChildren([
                Config.PayoutsByNode,
                Config.PayoutsSummary,
            ]),
            Config.Wallets.addChildren([
                Config.WalletDetails,
                Config.WalletsSummary,
            ]),
            Config.Bandwidth,
        ]),
        Config.Welcome,
        Config.AddFirstNode,
    ];
}

export const router = new Router(Config);

/**
 * List of allowed routes without any node added.
 */
const allowedRoutesNames = [Config.AddFirstNode.name, Config.Welcome.name];

/**
 * Checks if redirect to some of internal routes and no nodes added so far.
 * Redirect to Add first node screen if so.
 */
router.beforeEach(async(to, _from, next) => {

    if(to.path === '/') {
        next(Config.MyNodes);
    }

    if (store.state.nodes.nodes.length) {
        next();
    }

    if (!to.matched.some(record => allowedRoutesNames.includes(<string>record.name))) {
        await store.dispatch('nodes/fetch');

        if (!store.state.nodes.nodes.length) {
            next(Config.AddFirstNode);
        } else {
            next();
        }
    }

    next();
});
