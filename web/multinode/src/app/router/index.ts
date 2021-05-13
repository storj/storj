// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

import Router, { RouterMode } from 'vue-router';
import { Component } from 'vue-router/types/router';

import AddFirstNode from '@/app/views/AddFirstNode.vue';
import Dashboard from '@/app/views/Dashboard.vue';
import MyNodes from '@/app/views/MyNodes.vue';
import PayoutsByNode from '@/app/views/PayoutsByNode.vue';
import PayoutsPage from '@/app/views/PayoutsPage.vue';
import PayoutsRoot from '@/app/views/PayoutsRoot.vue';
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

    /**
     * default constructor.
     * @param path - route path.
     * @param name - name of the route, needed fot identifying route by name.
     * @param component - component mapped to route.
     * @param children - all nested components of current route.
     * @param meta - arbitrary information to routes like transition names, who can access the route, etc.
     */
    public constructor(path: string, name: string, component: Component, meta: Metadata | undefined = undefined) {
        this.path = path;
        this.name = name;
        this.component = component;
        this.meta = meta;
    }

    /**
     * Adds children routes to route.
     */
    public addChildren(children: Route[]): Route {
        this.children = children;

        return this;
    }

    public isChild(): boolean {
        return this.path[0] !== '/';
    }

    public with(child: Route): Route {
        if (!child.isChild()) {
            throw new Error('provided child root is not defined');
        }

        return new Route(`${this.path}/${child.path}`, child.name, child.component, child.meta);
    }
}

/**
 * Config contains configuration of all available routes for a Multinode Dashboard router.
 */
export class Config {
    public static Root: Route = new Route('/', 'Root', Dashboard, {requiresAuth: true});
    public static Welcome: Route = new Route('/welcome', 'Welcome', WelcomeScreen);
    public static AddFirstNode: Route = new Route('/add-first-node', 'AddFirstNode', AddFirstNode);
    public static MyNodes: Route = new Route('/my-nodes', 'MyNodes', MyNodes);
    public static Payouts: Route = new Route('/payouts', 'Payouts', PayoutsRoot);
    public static PayoutsSummary: Route = new Route('summary', 'PayoutsSummary', PayoutsPage);
    public static PayoutsByNode: Route = new Route('by-node/:id', 'PayoutsByNode', PayoutsByNode);

    public static mode: RouterMode = 'history';
    public static routes: Route[] = [
        Config.Root.addChildren([
            Config.MyNodes,
            Config.Payouts.addChildren([
                Config.PayoutsByNode,
                Config.PayoutsSummary,
            ]),
        ]),
        Config.Welcome,
        Config.AddFirstNode,
    ];
}

export const router = new Router(Config);
