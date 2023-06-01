# Multinode Dashboard

## Reference Articles

[Tech Preview Forum Post](https://forum.storj.io/t/tech-preview-multinode-dashboard-binaries/14572)

## Running the Multinode Dashboard in Docker with persistent data

Start the image as follows, while replacing the placeholder `<multinode-config-dir>` with a path to a directory. This directory will contain database files and a config.yaml.

```
docker run -d --restart unless-stopped \
    --user $(id -u):$(id -g) \
    -p 127.0.0.1:15002:15002/tcp \
    --mount type=bind,source="<multinode-config-dir>",destination=/app/config \
    --name multinode storjlabs/multinode:latest
```