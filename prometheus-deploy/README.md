## Docker-compose for running prometheus with imported data

This deployment configuration provide a docker-compose with volume bound to the [alibaba example](https://github.com/aleskandro/go-prometheus-backfiller/tree/master/examples/alibaba).

In particular, to use it with different data configure the volume mapping below:

```yaml
services:
  prometheus:
    ...
    volumes:
      - ../examples/alibaba/output:/prometheus # Configure me
```
