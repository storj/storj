// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

import Router, { RouterMode } from 'vue-router';
import { Component } from 'vue-router/types/router';

import Dashboard from '@/app/views/Dashboard.vue';

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
    public readonly children?: Route[];
    public readonly meta?: Metadata;

    /**
     * default constructor.
     * @param path - route path.
     * @param name - name of the route, needed fot identifying route by name.
     * @param component - component mapped to route.
     * @param children - all nested components of current route.
     * @param meta - arbitrary information to routes like transition names, who can access the route, etc.
     */
    public constructor(path: string, name: string, component: Component, children: Route[] | undefined = undefined, meta: Metadata | undefined = undefined) {
        this.path = path;
        this.name = name;
        this.component = component;
        this.children = children;
        this.meta = meta;
    }
}

/**
 * Config contains configuration of all available routes for a Multinode Dashboard router.
 */
export class Config {
    public static Root: Route = new Route('/', 'Root', Dashboard, undefined, {requiresAuth: true});

    public static mode: RouterMode = 'history';
    public static routes: Route[] = [
        Config.Root,
    ];
}

export const router = new Router(Config);
