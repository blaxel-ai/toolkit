FROM ubuntu:22.04

ENV DEBIAN_FRONTEND=noninteractive

# Install shells and curl (needed by install.sh)
RUN apt-get update && apt-get install -y \
    zsh \
    fish \
    curl \
    && rm -rf /var/lib/apt/lists/*

# Create a test user (install.sh installs to ~/.local/bin by default)
RUN useradd -m -s /bin/bash testuser
USER testuser
WORKDIR /home/testuser

# Copy install.sh and test script
COPY --chown=testuser:testuser install.sh /home/testuser/install.sh
COPY --chown=testuser:testuser test/install/test_install.sh /home/testuser/test_install.sh
RUN chmod +x /home/testuser/install.sh /home/testuser/test_install.sh

ENTRYPOINT ["/bin/bash", "/home/testuser/test_install.sh"]
