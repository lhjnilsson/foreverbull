# Check if docker exists and is running
import json
import subprocess


try:
    result = subprocess.run(["docker", "version", "-f", "json"], capture_output=True, text=True)
    docker_info = json.loads(result.stdout)
except subprocess.CalledProcessError:
    print("Docker is not running")
    print("Please make sure docker is installed and running")
    exit(1)
