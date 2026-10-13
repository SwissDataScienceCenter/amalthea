FROM rclone/rclone:1.75 AS rclone

# Using the debug image because it contains busybox which adds a shell.
# We need a shell to run the preStop checks on Rclone that pause the shutdown in case there
# are any in-progress file transfers in the Rclone server.
FROM gcr.io/distroless/static-debian12:debug-nonroot
COPY --from=rclone /usr/local/bin/rclone /usr/local/bin/rclone
ENV XDG_CONFIG_HOME=/config
ENTRYPOINT ["/usr/local/bin/rclone"]
