This folder contains rolling upgrade tests.

1. It first sets up an old satellite.
2. Uploads some data
3. Migrate database and configuration files
4. Run half-and-half latest-release and tip of storagenodes
5. Test upload/download with both old and new satellite-api