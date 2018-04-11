# Storj Dependencies

We should attempt to limit the dependencies we pull in for our projects in order to ease switching between projects.

These are only suggestions based on previous experience and our goals with this platform. If we have a need arise or a better solution is found we should consider and weigh the cost of using that solution.

[The twelve-factor app methodology](https://12factor.net/)

[Go in Production](https://peter.bourgon.org/go-in-production/)
### HTTP

[HTTP](https://golang.org/pkg/net/http/)

[Routing](https://github.com/julienschmidt/httprouter)

### Logging
    
[Uber Zap](https://github.com/uber-go/zap)
### Metrics

[Space Monkey Monkit](https://github.com/spacemonkeygo/monkit/)

### CLI

[urfave/cli](https://github.com/urfave/cli)

### Testing

[testing](https://golang.org/pkg/testing/)

[assertions](https://github.com/stretchr/testify)

### Configuration

[viper](https://github.com/spf13/viper)

### Error Handling

[zeebo](https://github.com/zeebo/errs)


