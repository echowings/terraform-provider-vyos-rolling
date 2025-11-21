FROM docker.io/vyos/vyos-build:current

# Run as root to install runtime library packages that the build may expect.
USER root

# Try installing likely package names; allow failures to keep image build robust.
RUN apt-get update && \
    apt-get install -y --no-install-recommends libvyosconfig0 vyos-config ca-certificates || true && \
    apt-get clean && rm -rf /var/lib/apt/lists/*

# Default to sh so `make` commands executed via `docker run` still work.
CMD ["/bin/sh"]
