# CWSO Proxmox LXC Deployment Guide

**Version:** 1.0
**Last updated:** 2026-06-28
**Environment:** Proxmox VE 7.0+ with LXC containers
**Minimum resources:** 4GB RAM, 20GB storage per container

> **⚠️ Not yet validated end-to-end.** This guide was written from the original deployment plan
> (`docs/plans/plan-013-cwso-deployment-guides.md`) but has not been run against a real
> Proxmox host since. Unlike `local-docker-desktop-guide.md` (validated and
> corrected in `docs/tasks/task-T313.md`), no task in this repo’s history confirms this guide
> works as written. Treat it as a starting point, not a proven procedure, until a future task
> validates it for real and removes this notice.

---

## Quick Start (15 minutes)

Prerequisites: Proxmox VE running, SSH access to Proxmox host

```bash
# 1. Copy deployment script to Proxmox host
scp scripts/deploy/cwso-proxmox-setup.sh root@proxmox-host:/tmp/

# 2. SSH to Proxmox host and run setup
ssh root@proxmox-host "bash /tmp/cwso-proxmox-setup.sh --auto"

# 3. SSH to new container and verify
ssh root@proxmox-host "pct enter $(pct list | grep cwso | awk '{print $1}')"
curl http://localhost:8080/health

# Done! CWSO is running in Proxmox
```

---

## Prerequisites

### On Proxmox Host
- Proxmox VE 7.0 or later
- SSH access with root or sudo privileges
- Minimum 4GB RAM available
- Minimum 20GB free storage
- Network access to NTP server (for time sync)

### Container Requirements
- Ubuntu 20.04 LTS or Debian 11 (recommended)
- 2 CPU cores minimum
- 4GB RAM minimum
- 10GB root disk + 10GB for Parquet store

### Network Planning
- Determine container IP address (static recommended)
- Decide on network bridge (vmbr0 typical)
- Plan port accessibility (internal or external)
- Firewall rules for ports 8080, 8787, 8788, 8789

---

## Architecture

```
Proxmox Host
├─ vmbr0 (Bridge)
│  └─ CWSO LXC Container (192.168.1.100)
│     ├─ Orchestrator (8080)
│     ├─ Rollout Proxy (8787)
│     ├─ Git Shadow (8788)
│     └─ Merge Engine (8789)
└─ Storage
   ├─ Container filesystem
   └─ Parquet store (/var/lib/cwso/parquet)
```

---

## Step-by-Step Setup

### Step 1: Prepare Proxmox Host

```bash
# Connect to Proxmox host
ssh root@proxmox-host

# Update system
apt update && apt upgrade -y

# Verify Docker support
grep -i docker /proc/cpuinfo  # Should show virtualization flags
lsmod | grep kvm              # Should show kvm loaded

# Install necessary tools
apt install -y git curl wget

# Create deployment directory
mkdir -p /opt/cwso-deploy
cd /opt/cwso-deploy
```

### Step 2: Determine Container Configuration

```bash
# Choose container ID (e.g., 200-299 for CWSO)
CTID=201

# Choose hostname
HOSTNAME="cwso-prod-01"

# Choose IP address (check your network range)
IP_ADDRESS="192.168.1.100/24"
GATEWAY="192.168.1.1"

# Save for reference
cat > /opt/cwso-deploy/container-config.txt << EOF
CTID: $CTID
HOSTNAME: $HOSTNAME
IP: $IP_ADDRESS
GATEWAY: $GATEWAY
ROOTFS: /var/lib/lxc/$CTID/rootfs
EOF
```

### Step 3: Create LXC Container

```bash
# Method 1: Using Proxmox CLI
pct create 201 \
  local:vztmpl/ubuntu-20.04-standard_20.04-1_amd64.tar.zst \
  --hostname cwso-prod-01 \
  --cores 2 \
  --memory 4096 \
  --swap 2048 \
  --storage local \
  --net0 name=eth0,bridge=vmbr0,ip=192.168.1.100/24,gw=192.168.1.1

# Method 2: Using provided setup script (see below)
```

### Step 4: Configure Container Networking

```bash
# Start container
pct start 201

# Enter container
pct enter 201

# Inside container - configure networking
cat >> /etc/network/interfaces << EOF
auto eth0
iface eth0 inet static
  address 192.168.1.100/24
  gateway 192.168.1.1
  dns-nameservers 8.8.8.8 8.8.4.4
EOF

# Restart networking
systemctl restart networking

# Verify connectivity
ping 8.8.8.8  # Should succeed
```

### Step 5: Install Docker in Container

```bash
# Inside container - update system
apt update && apt upgrade -y

# Install Docker
curl -fsSL https://get.docker.com -o get-docker.sh
bash get-docker.sh

# Enable Docker service
systemctl enable docker
systemctl start docker

# Verify Docker
docker --version
docker info
```

### Step 6: Deploy CWSO Components

```bash
# Copy CWSO deployment to container
scp -r /path/to/emage.code/deploy/t226-phase2.env root@192.168.1.100:/opt/cwso/

# Inside container - create deployment directory
mkdir -p /opt/cwso/data

# Copy docker-compose file
# (Or pull from repository)
git clone https://gitlab.com/em-age/emage.code.git /opt/cwso/repo
cd /opt/cwso/repo

# Start services
docker-compose -f deploy/docker-compose-t226.yml up -d

# Verify
docker ps
docker-compose logs
```

### Step 7: Configure Storage for Parquet

```bash
# On Proxmox host - create storage mount for Parquet
# Inside container - create mount point
mkdir -p /var/lib/cwso/parquet

# Configure persistence
# Add to docker-compose or create named volume
docker volume create cwso-parquet-store

# Mount in containers
# (Configure in docker-compose: volumes section)
```

---

## Using the Automated Setup Script

### Download Script

```bash
# On Proxmox host
scp scripts/deploy/cwso-proxmox-setup.sh root@proxmox-host:/tmp/

# Or via curl
ssh root@proxmox-host "curl -o /tmp/cwso-proxmox-setup.sh https://path-to-script"
```

### Run Setup

```bash
# Interactive setup (ask for all parameters)
bash /tmp/cwso-proxmox-setup.sh

# Auto setup (use defaults)
bash /tmp/cwso-proxmox-setup.sh --auto

# Custom configuration
bash /tmp/cwso-proxmox-setup.sh \
  --ctid 201 \
  --hostname cwso-prod-01 \
  --ip 192.168.1.100/24 \
  --gateway 192.168.1.1 \
  --cores 2 \
  --memory 4096 \
  --non-interactive
```

### Script Options

```
--ctid <ID>              Container ID (default: 201)
--hostname <name>        Container hostname (default: cwso-prod-01)
--ip <addr/mask>         Container IP (default: 192.168.1.100/24)
--gateway <ip>           Gateway IP (default: 192.168.1.1)
--cores <n>              CPU cores (default: 2)
--memory <MB>            Memory in MB (default: 4096)
--storage <pool>         Storage pool (default: local)
--bridge <name>          Network bridge (default: vmbr0)
--template <path>        LXC template path (default: Ubuntu 20.04)
--auto                   Use all defaults
--help                   Show help
```

---

## Post-Deployment Configuration

### Firewall Rules (If Needed)

```bash
# On Proxmox host - allow traffic to container ports
ufw allow from any to 192.168.1.100 port 8080 proto tcp
ufw allow from any to 192.168.1.100 port 8787 proto tcp
ufw allow from any to 192.168.1.100 port 8788 proto tcp
ufw allow from any to 192.168.1.100 port 8789 proto tcp
```

### DNS Configuration

```bash
# Inside container - configure DNS
# Edit /etc/resolv.conf or /etc/netplan/
nameserver 8.8.8.8
nameserver 8.8.4.4

# Or configure in Proxmox host DHCP/DNS
```

### Resource Limits

```bash
# Adjust memory limits
pct set 201 --memory 6144  # Increase to 6GB

# Adjust CPU allocation
pct set 201 --cores 4      # Allocate 4 cores

# Adjust swap
pct set 201 --swap 4096    # Increase swap
```

---

## Backup and Restore

### Backup Container

```bash
# On Proxmox host
# Full container backup
vzdump 201 --storage <backup-storage>

# Or specific backup directory
mkdir -p /var/backups/cwso
vzdump 201 --dumpdir /var/backups/cwso

# Scheduled backup via Proxmox web UI
# Or cron job
0 2 * * * /usr/sbin/vzdump 201 --dumpdir /var/backups/cwso
```

### Restore Container

```bash
# List available backups
ls -la /var/backups/cwso/

# Restore backup (destructive - replaces current)
pct stop 201
qmrestore /var/backups/cwso/dump-201-*.tar.zst 201
pct start 201
```

---

## Monitoring and Management

### Container Status

```bash
# On Proxmox host
pct status 201
pct logs 201 --lines 50

# List containers
pct list

# Resource usage
proxmox-ve-status  # or check web UI
```

### Inside Container

```bash
# SSH into container
pct enter 201

# Check services
docker-compose ps
docker stats

# View logs
docker-compose logs -f orchestrator

# Check disk usage
df -h
```

### Health Checks

```bash
# From Proxmox host
ssh 192.168.1.100 "curl http://localhost:8080/health"

# Or from container
curl http://localhost:8080/health
curl http://localhost:8787/health
```

---

## Troubleshooting

### Container Won't Start

```bash
# Check logs
pct logs 201

# Verify Proxmox resources
top  # Check CPU/memory on host

# Check container configuration
pct config 201

# Try manual start with debug
pct start 201 --verbose
```

### Network Connectivity Issues

```bash
# Inside container - check networking
ip addr show
ip route show

# Ping gateway
ping 192.168.1.1

# Check DNS
cat /etc/resolv.conf
ping 8.8.8.8

# On Proxmox host - check bridge
brctl show vmbr0
```

### Docker Issues

```bash
# Inside container - check Docker
systemctl status docker
docker ps
docker ps -a

# View Docker logs
journalctl -u docker -n 50

# Restart Docker
systemctl restart docker
```

### Storage/Disk Space

```bash
# Inside container - check disk
df -h
du -sh /var/lib/cwso/parquet

# On Proxmox host - check container storage
pvs
lvs

# Expand container storage
pct resize 201 rootfs +10G
```

### Port Conflicts

```bash
# Check open ports
netstat -tulpn | grep LISTEN
# or
ss -tulpn | grep LISTEN

# Change port mappings in docker-compose.yml
# Then restart
docker-compose restart
```

---

## Advanced Configuration

### Multi-Container Setup

For high availability, deploy multiple CWSO instances:

```bash
# Container 1 (202)
pct create 202 ...

# Container 2 (203)
pct create 203 ...

# Configure load balancer on host
# Use HAProxy or Nginx as reverse proxy
```

### Persistent Storage

```bash
# Create persistent Parquet storage mount
mkdir -p /mnt/cwso-parquet
mount -t nfs nfs-server:/export/cwso /mnt/cwso-parquet

# Mount in container
pct set 201 --mp0 /mnt/cwso-parquet,mp=/mnt/cwso-parquet
```

### Resource Templates

Save container configuration for easy recreation:

```bash
# Export container template
pct dump 201 > /opt/cwso-templates/cwso-template-v1.conf

# Create new container from template
pct restore <vmid> /opt/cwso-templates/cwso-template-v1.conf
```

---

## Maintenance

### Regular Tasks

```bash
# Weekly - check logs for errors
pct enter 201 "docker-compose logs | grep -i error"

# Monthly - update container
pct enter 201 "apt update && apt upgrade -y"

# Monthly - backup container
vzdump 201 --dumpdir /var/backups/cwso

# Quarterly - review resource usage
proxmox resource report
```

### Container Lifecycle

```bash
# Stop container (preserves data)
pct stop 201

# Start container
pct start 201

# Restart container
pct reboot 201

# Destroy container (⚠️ deletes all data)
pct destroy 201

# Migrate container to different storage/node
pct move-disk 201 --storage <new-storage>
```

---

## Next Steps

### After Deployment
1. ✅ Verify container is running: `pct status 201`
2. ✅ Test health endpoints: `ssh 192.168.1.100 "curl http://localhost:8080/health"`
3. ✅ Review logs for any errors
4. ✅ Configure backup schedule
5. ✅ Document container details

### For Production
- Set up monitoring alerts
- Configure automated backups
- Test disaster recovery procedure
- Document runbook for operations team

### Additional Deployment Options
- See [Docker Desktop Guide](local-docker-desktop-guide.md) for development
- See [GCP Cloud Run Guide](gcp-cloud-run-guide.md) for cloud deployment

---

## Support

For issues:
1. Check troubleshooting section
2. See [Deployment Troubleshooting Guide](troubleshooting-guide.md)
3. Review Proxmox logs: `journalctl -xe`
4. Check container logs: `pct logs 201`
5. Consult Proxmox documentation: https://pve.proxmox.com/wiki/

---

## Appendix: Container Template

Default container configuration:
```
Template: Ubuntu 20.04 LTS
Cores: 2
Memory: 4GB
Swap: 2GB
Storage: local
Hostname: cwso-prod-01
Network: vmbr0 (bridge)
IP: 192.168.1.100/24
```

Adjust as needed for your environment.
