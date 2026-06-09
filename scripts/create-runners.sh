#!/bin/bash
# Must be run as root to create users and install systemd services

# --- CONFIGURATION ---
GITHUB_URL="https://github.com/Technarion-Oy/thaw" # Repository or Organization URL
RUNNER_VERSION="2.334.0"                           # Check GitHub for the latest runner release!
RUNNER_USER="github-runner"
NUM_RUNNERS=6

# Prompt securely for the runner token
read -s -p "Enter your fresh GitHub Runner Token: " RUNNER_TOKEN
echo "" # Adds a newline since the silent prompt suppresses the 'Enter' keystroke
# ---------------------

echo "Starting setup for $NUM_RUNNERS self-hosted runners..."

# 1. Create the dedicated, unprivileged runner user
if ! id -u "$RUNNER_USER" >/dev/null 2>&1; then
    echo "Creating user $RUNNER_USER..."
    useradd -m -s /bin/bash "$RUNNER_USER"
    
    # The user must be in the docker group to spin up the container: ubuntu:24.04
    usermod -aG docker "$RUNNER_USER" 
fi

# 2. Create the hygienic cleanup hook
# This script runs AFTER every job. It mounts the runner's _work directory
# into a throwaway alpine container to forcefully delete root-owned files left by the job.
HOOK_SCRIPT="/home/$RUNNER_USER/cleanup-hook.sh"
echo "Creating cleanup hook..."

cat << 'EOF' > "$HOOK_SCRIPT"
#!/bin/bash
# The hook executes from the runner's root directory.
# Mount the _work directory and wipe it clean.
docker run --rm -v "$(pwd)/_work:/runner_work" alpine sh -c 'rm -rf /runner_work/* /runner_work/.* 2>/dev/null || true'
echo "Workspace hygienically wiped."
EOF

chmod +x "$HOOK_SCRIPT"
chown "$RUNNER_USER:$RUNNER_USER" "$HOOK_SCRIPT"

# 3. Install and configure the runners
for i in $(seq 1 $NUM_RUNNERS); do
    RUNNER_DIR="/home/$RUNNER_USER/runner-$i"
    echo "Configuring runner $i in $RUNNER_DIR..."
    
    mkdir -p "$RUNNER_DIR"
    cd "$RUNNER_DIR" || exit

    # Download and extract runner binaries
    curl -o actions-runner-linux-x64.tar.gz -L "https://github.com/actions/runner/releases/download/v${RUNNER_VERSION}/actions-runner-linux-x64-${RUNNER_VERSION}.tar.gz"
    tar xzf ./actions-runner-linux-x64.tar.gz
    rm actions-runner-linux-x64.tar.gz

    # Ensure the unprivileged user owns the runner files
    chown -R "$RUNNER_USER:$RUNNER_USER" "$RUNNER_DIR"

    # Configure runner as the unprivileged user
    sudo -u "$RUNNER_USER" ./config.sh \
        --url "$GITHUB_URL" \
        --token "$RUNNER_TOKEN" \
        --name "$(hostname)-runner-$i" \
        --unattended \
        --replace

    # Register the cleanup hook in the runner's environment
    echo "ACTIONS_RUNNER_HOOK_JOB_COMPLETED=$HOOK_SCRIPT" | sudo -u "$RUNNER_USER" tee -a .env > /dev/null

    # Install systemd service (this requires root, but the service is configured to run as RUNNER_USER)
    ./svc.sh install "$RUNNER_USER"
    
    # Start the service
    ./svc.sh start
done

echo "Successfully configured $NUM_RUNNERS hygienic runners!"
