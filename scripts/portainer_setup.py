#!/usr/bin/env python3
"""
Portainer Container Setup Script

Creates PostgreSQL and NATS containers for the Kootenai platform using Portainer API.

Usage:
    python3 portainer_setup.py
"""

import json
import os
import urllib.request
import urllib.error

# Portainer configuration (all from environment variables)
PORTAINER_URL = os.environ.get("PORTAINER_URL", "http://<INFRA_VM_IP>:9000")
PORTAINER_USER = os.environ.get("PORTAINER_USER", "automation")
PORTAINER_PASSWORD = os.environ.get("PORTAINER_PASSWORD")
if not PORTAINER_PASSWORD:
    raise SystemExit("ERROR: PORTAINER_PASSWORD environment variable is required")

# Container configurations
_db_password = os.environ.get("DATABASE_PASSWORD")
if not _db_password:
    raise SystemExit("ERROR: DATABASE_PASSWORD environment variable is required")

POSTGRES_CONFIG = {
    "name": "kootenai-postgres",
    "image": "postgres:16-alpine",
    "env": [
        "POSTGRES_USER=labadmin",
        f"POSTGRES_PASSWORD={_db_password}",
        "POSTGRES_DB=virtuallab"
    ],
    "ports": [{"host": 5432, "container": 5432}],
    "volumes": [{"name": "kootenai-postgres-data", "container": "/var/lib/postgresql/data"}],
    "restart_policy": "unless-stopped"
}

NATS_CONFIG = {
    "name": "kootenai-nats",
    "image": "nats:2.10-alpine",
    "command": ["--jetstream", "--store_dir", "/data"],
    "ports": [
        {"host": 4222, "container": 4222},
        {"host": 8222, "container": 8222}
    ],
    "volumes": [{"name": "kootenai-nats-data", "container": "/data"}],
    "restart_policy": "unless-stopped"
}


class PortainerClient:
    def __init__(self, url, username, password):
        self.url = url
        self.token = None
        self._authenticate(username, password)

    def _authenticate(self, username, password):
        """Authenticate to Portainer and get JWT token."""
        data = json.dumps({"username": username, "password": password}).encode('utf-8')
        req = urllib.request.Request(
            f"{self.url}/api/auth",
            data=data,
            method="POST"
        )
        req.add_header("Content-Type", "application/json")

        with urllib.request.urlopen(req) as response:
            result = json.loads(response.read().decode('utf-8'))
            self.token = result["jwt"]

    def _request(self, path, method="GET", data=None, timeout=30):
        """Make an authenticated request to Portainer API."""
        url = f"{self.url}{path}"
        req = urllib.request.Request(url, method=method)
        req.add_header("Authorization", f"Bearer {self.token}")

        if data is not None:
            req.add_header("Content-Type", "application/json")
            req.data = json.dumps(data).encode('utf-8')

        try:
            with urllib.request.urlopen(req, timeout=timeout) as response:
                content = response.read()
                if content:
                    return json.loads(content.decode('utf-8'))
                return None
        except urllib.error.HTTPError as e:
            error_body = e.read().decode('utf-8')
            raise Exception(f"HTTP {e.code}: {error_body}")

    def _request_stream(self, path, method="POST", timeout=120):
        """Make a streaming request (for image pulls)."""
        url = f"{self.url}{path}"
        req = urllib.request.Request(url, method=method)
        req.add_header("Authorization", f"Bearer {self.token}")

        try:
            with urllib.request.urlopen(req, timeout=timeout) as response:
                while True:
                    line = response.readline()
                    if not line:
                        break
        except urllib.error.HTTPError as e:
            # 404 or other errors during pull are usually okay (image might exist)
            pass

    def get_endpoints(self):
        """Get list of Docker endpoints."""
        return self._request("/api/endpoints")

    def pull_image(self, endpoint_id, image):
        """Pull a Docker image."""
        self._request_stream(
            f"/api/endpoints/{endpoint_id}/docker/images/create?fromImage={image}",
            timeout=180
        )

    def create_container(self, endpoint_id, name, config):
        """Create a Docker container."""
        body = {
            "Image": config["image"],
            "Env": config.get("env", []),
            "ExposedPorts": {},
            "HostConfig": {
                "PortBindings": {},
                "Binds": [],
                "RestartPolicy": {
                    "Name": config.get("restart_policy", "no")
                }
            }
        }

        if "command" in config:
            body["Cmd"] = config["command"]

        for port_mapping in config.get("ports", []):
            container_port = f"{port_mapping['container']}/tcp"
            body["ExposedPorts"][container_port] = {}
            body["HostConfig"]["PortBindings"][container_port] = [
                {"HostIp": "0.0.0.0", "HostPort": str(port_mapping["host"])}
            ]

        for volume in config.get("volumes", []):
            bind = f"{volume['name']}:{volume['container']}"
            body["HostConfig"]["Binds"].append(bind)

        return self._request(
            f"/api/endpoints/{endpoint_id}/docker/containers/create?name={name}",
            method="POST",
            data=body
        )

    def start_container(self, endpoint_id, container_name):
        """Start a Docker container."""
        url = f"{self.url}/api/endpoints/{endpoint_id}/docker/containers/{container_name}/start"
        req = urllib.request.Request(url, method="POST")
        req.add_header("Authorization", f"Bearer {self.token}")

        try:
            with urllib.request.urlopen(req) as response:
                return True
        except urllib.error.HTTPError as e:
            if e.code == 304:
                return True  # Already running
            raise

    def container_exists(self, endpoint_id, container_name):
        """Check if a container exists."""
        try:
            self._request(f"/api/endpoints/{endpoint_id}/docker/containers/{container_name}/json")
            return True
        except Exception:
            return False


def create_container(client, endpoint_id, config):
    """Create and start a container."""
    name = config["name"]

    # Check if container already exists
    if client.container_exists(endpoint_id, name):
        print(f"  Container {name} already exists")
        print(f"  Starting container...")
        try:
            client.start_container(endpoint_id, name)
            print(f"  Container started (or already running)")
        except Exception as e:
            print(f"  Warning: {e}")
        return

    # Pull image
    print(f"  Pulling image: {config['image']}...")
    try:
        client.pull_image(endpoint_id, config["image"])
        print(f"  Image pulled successfully")
    except Exception as e:
        print(f"  Warning during pull: {e}")

    # Create container
    print(f"  Creating container: {name}...")
    try:
        result = client.create_container(endpoint_id, name, config)
        container_id = result.get("Id", "")[:12] if result else "unknown"
        print(f"  Container created with ID: {container_id}")
    except Exception as e:
        if "already in use" in str(e).lower() or "conflict" in str(e).lower():
            print(f"  Container already exists")
        else:
            raise

    # Start container
    print(f"  Starting container...")
    client.start_container(endpoint_id, name)
    print(f"  Container started successfully")


def main():
    print("=" * 60)
    print("Portainer Container Setup")
    print("=" * 60)
    print(f"Portainer URL: {PORTAINER_URL}")
    print("=" * 60)

    # Connect to Portainer
    print("\nAuthenticating to Portainer...")
    try:
        client = PortainerClient(PORTAINER_URL, PORTAINER_USER, PORTAINER_PASSWORD)
        print("Authentication successful!")
    except Exception as e:
        print(f"Authentication failed: {e}")
        return 1

    # Get endpoints
    print("\nFetching Docker endpoints...")
    try:
        endpoints = client.get_endpoints()
        if not endpoints:
            print("No Docker endpoints found in Portainer!")
            return 1

        endpoint_id = endpoints[0]["Id"]
        endpoint_name = endpoints[0]["Name"]
        print(f"Using endpoint: {endpoint_name} (ID: {endpoint_id})")
    except Exception as e:
        print(f"Failed to get endpoints: {e}")
        return 1

    # Create PostgreSQL container
    print(f"\n=== Creating {POSTGRES_CONFIG['name']} ===")
    try:
        create_container(client, endpoint_id, POSTGRES_CONFIG)
        print(f"PostgreSQL container ready!")
    except Exception as e:
        print(f"Failed to create PostgreSQL container: {e}")

    # Create NATS container
    print(f"\n=== Creating {NATS_CONFIG['name']} ===")
    try:
        create_container(client, endpoint_id, NATS_CONFIG)
        print(f"NATS container ready!")
    except Exception as e:
        print(f"Failed to create NATS container: {e}")

    print("\n" + "=" * 60)
    print("Container setup complete!")
    print("=" * 60)
    print("\nConnection details:")
    print(f"  PostgreSQL: <INFRA_VM_IP>:5432")
    print(f"    Database: virtuallab")
    print(f"    User: labadmin")
    print(f"    Password: (see DATABASE_PASSWORD env var)")
    print(f"  NATS: nats://<INFRA_VM_IP>:4222")
    print(f"  NATS Monitoring: http://<INFRA_VM_IP>:8222")
    print("=" * 60)

    return 0


if __name__ == "__main__":
    exit(main())
