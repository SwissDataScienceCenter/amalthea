FROM rclone/rclone:1.75 AS rclone

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=rclone /usr/local/bin/rclone /usr/local/bin/rclone
ENV XDG_CONFIG_HOME=/config
ENTRYPOINT ["/usr/local/bin/rclone"]
