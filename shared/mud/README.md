# mud

mud is a package for lazily starting up a large set of services and chores.
The `mud` package name comes from a common architectural pattern called "Big Ball of Mud".

You can think about mud as a very huge map of service instances: `map[type]component`.
Where component includes the singleton instance and functions to initialize the singleton and run/close them.

Components can also depend on each other, and there are helper function to filter the required components (and/or
initialize/start them).

Compared to other similar libraries, like https://github.com/uber-go/fx or https://github.com/uber-go/dig, mud is just a
very flexible framework, it wouldn't like to restrict the usage. Therefore advanced workflows also can be implemented
with filtering different graphs and using them.

Users of this library has more power (and more responsibility).

## Getting started

You can create the instance registry with:

```
mud := mud.NewBall()
```

Register a new component:

```
Provide[your.Service](ball, func() your.Service {
    return your.NewService()
})
```

Now, your component is registered, but not yet initialized. You should select some of the services to Init / run them:

```
err := mud.ForEach(ball, mud.Initialize(context.TODO()))
if err != nil {
   panic(err)
}
```

Now your component is initialized:

```
fmt.Println(mud.Find(ball, mud.All)[0].Instance())
```

This one selected the first component (we registered only one), but you can also use different selectors. This one
selects the components by type.

```
fmt.Println(mud.Find(ball, mud.Select[your.Service](ball))[0].Instance())
```

Or, of you are sure, it's there:

```
fmt.Println(mud.MustLookup[your.Service](ball))
```

## Dependencies and dependency injection

Dependencies are automatically injected. Let's say you have two structs:

```
type Service struct {
}

func NewService() Service {
	return Service{}
}

type Endpoint struct {
	Service Service
}

func NewEndpoint(service Service) Endpoint {
	return Endpoint{
		Service: service,
	}
}
```

Now you can register both:

```
mud.Provide[your.Service](ball, your.NewService)
mud.Provide[your.Endpoint](ball, your.NewEndpoint)
```

When you initialize the Endpoint, Service will be injected (if the instance is available!!!):

```
err := mud.MustDo[your.Service](ball, mud.Initialize(context.TODO()))
if err != nil {
    panic(err)
}

err = mud.MustDo[your.Endpoint](ball, mud.Initialize(context.TODO()))
if err != nil {
    panic(err)
}
```

But instead of initializing manually, you can also just ask what you need, and initialize everything in the right order

```
err := mud.ForEachDependency(ball, mud.Select[your.Endpoint](ball), mud.Initialize(context.TODO()), mud.All)
```

## Views

Views are useful when you already have sg. registered, but you would like to make it fully or partially available under
different type:

```
mud.Provide[satellite.DB](ball, OpenSatelliteDB)
mud.View[satellite.DB, gracefulexit.DB](ball, satellite.DB.GracefulExit)
```

This registers a `satellite.DB` (first line) and a `gracefulexit.DB` (second line). And if `gracefulexit.DB` is needed
for injection, it will call the function to get it.  