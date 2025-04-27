# Check if docker exists and is running
import json
import subprocess


try:
    result = subprocess.run(["docker", "version", "-f", "json"], capture_output=True, text=True)
    docker_info = json.loads(result.stdout)
    print(docker_info)
    print("Docker is running")
except subprocess.CalledProcessError:
    print("Docker is not running")
    exit(1)
