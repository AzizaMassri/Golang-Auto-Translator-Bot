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

cd /opt/GlobalSync-Bot

# 5.1 Create env file from template if needed
if [ ! -f ".env" ]; then
    if [ -f ".env.example" ]; then
        echo "Creating /opt/GlobalSync-Bot/.env from .env.example template..."
        cp .env.example .env
    else
        echo "Creating basic /opt/GlobalSync-Bot/.env (no .env.example found)..."
        touch .env
    fi
fi

# 5.1.1 Port selection for shared VM environments
port_in_use() {
    local port=$1
    ss -ltn "sport = :$port" | grep -q LISTEN
}

find_available_port() {
    for port in $(seq 8080 8090); do
        if ! port_in_use "$port"; then
            echo "$port"
            return 0
        fi
    done
    return 1
}

set_env_port() {
    local new_port=$1
    if grep -q '^PORT=' .env 2>/dev/null; then
        sed -i 's/^PORT=.*/PORT="'$new_port'"/' .env
    else
        echo 'PORT="'$new_port'"' >> .env
    fi
}

current_port=$(grep -E '^PORT=' .env 2>/dev/null | cut -d= -f2- | tr -d '"\'' | tr -d '[:space:]')
if [ -z "$current_port" ]; then
    current_port=8080
    set_env_port "$current_port"
fi

if port_in_use "$current_port"; then
    echo "Port $current_port is already in use. Selecting an alternate port for GlobalSync..."
    available_port=$(find_available_port)
    if [ -z "$available_port" ]; then
        echo "ERROR: No available ports found in the 8080-8090 range. Please free a port or update /opt/GlobalSync-Bot/.env manually."
        exit 1
    fi
    echo "Using port $available_port for GlobalSync. Updating /opt/GlobalSync-Bot/.env..."
    set_env_port "$available_port"
    current_port=$available_port
fi

echo "GlobalSync will bind to PORT=$current_port"

sudo chown -R $USER:$USER /opt/GlobalSync-Bot

check_port() {
    local port=$1
    if ss -ltn "sport = :$port" | grep -q LISTEN; then
        echo "WARNING: port $port is already in use. If KingBot or another service uses the same port, this may conflict."
    fi
}

check_port 6379
check_port 4222
check_port 9090
check_port "$current_port"

# 5.2 Start the local infrastructure stack
echo "[5.2/6] Starting required Docker infrastructure..."
sudo docker compose -f /opt/GlobalSync-Bot/docker-compose.yml up -d

# 5.3 Build the native Go binary
echo "[5.3/6] Compiling GlobalSync native binary..."
mkdir -p /opt/GlobalSync-Bot/bin
cd /opt/GlobalSync-Bot
go build -o bin/globalsync ./cmd/globalsync

# 6. Bind System Daemon
echo "[6/6] Linking highly-available System Daemon..."
sudo ln -sf /opt/GlobalSync-Bot/scripts/globalsync.service /etc/systemd/system/globalsync.service
sudo systemctl daemon-reload
sudo systemctl enable globalsync.service
sudo systemctl start globalsync.service

echo "======================================"
echo " VM Installation Complete!            "
echo "======================================"
echo "Next Steps:"
echo "1. Run 'source /etc/profile' to apply Go to your current terminal."
echo "2. Edit your environment variables in /opt/GlobalSync-Bot/.env"
echo "3. Run 'cd /opt/GlobalSync-Bot && ./scripts/deploy_vm.sh' to launch!"
