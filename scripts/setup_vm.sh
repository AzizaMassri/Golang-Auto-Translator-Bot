#!/bin/bash
# setup_vm.sh
# One-click installation script for a fresh Ubuntu/Debian VM deployment.

set -e # Exit immediately if a command exits with a non-zero status

echo "======================================"
echo "    GlobalSync Bot VM Propagator    "
echo "======================================"

# 1. Update system packages
echo "[1/6] Updating package manager..."
sudo apt-get update -y
sudo apt-get upgrade -y
sudo apt-get install -y curl wget git unzip

# 2. Install Docker
echo "[2/6] Installing Docker..."
if ! command -v docker &> /dev/null; then
    curl -fsSL https://get.docker.com -o get-docker.sh
    sudo sh get-docker.sh
    rm get-docker.sh
    sudo usermod -aG docker $USER
else
    echo "Docker already installed!"
fi

# 3. Install Docker-Compose plugin (latest method)
echo "[3/6] Installing Docker Compose..."
if ! command -v docker-compose &> /dev/null; then
    sudo apt-get install -y docker-compose-plugin docker-compose
fi

# 4. Install Go (Golang)
echo "[4/6] Installing Go 1.21.6..."
if ! command -v go &> /dev/null; then
    wget https://go.dev/dl/go1.21.6.linux-amd64.tar.gz
    sudo rm -rf /usr/local/go
    sudo tar -C /usr/local -xzf go1.21.6.linux-amd64.tar.gz
    rm go1.21.6.linux-amd64.tar.gz
    
    # Add to path for all users
    echo "export PATH=$PATH:/usr/local/go/bin" | sudo tee -a /etc/profile
    export PATH=$PATH:/usr/local/go/bin
else
    echo "Go already installed!"
fi

# 5. Clone Repository
echo "[5/6] Bootstrapping GlobalSync Workspace..."
if [ ! -d "/opt/GlobalSync-Bot" ]; then
    sudo git clone https://github.com/ROCKETexecdevs/GlobalSync-Bot.git /opt/GlobalSync-Bot
    sudo chown -R $USER:$USER /opt/GlobalSync-Bot
else
    echo "Workspace already exists at /opt/GlobalSync-Bot. Pulling latest..."
    cd /opt/GlobalSync-Bot
    git pull origin main
fi

# 6. Bind System Daemon
echo "[6/6] Linking highly-available System Daemon..."
sudo ln -sf /opt/GlobalSync-Bot/scripts/globalsync.service /etc/systemd/system/globalsync.service
sudo systemctl daemon-reload
sudo systemctl enable globalsync.service

echo "======================================"
echo " VM Installation Complete!            "
echo "======================================"
echo "Next Steps:"
echo "1. Run 'source /etc/profile' to apply Go to your current terminal."
echo "2. Edit your environment variables in /opt/GlobalSync-Bot/.env"
echo "3. Run 'cd /opt/GlobalSync-Bot && ./scripts/deploy_vm.sh' to launch!"
