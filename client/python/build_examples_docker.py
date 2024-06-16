import os
import subprocess

files = []
basename = "lhjnilsson/example-{algo}"

for filename in os.listdir("examples"):
    if filename.endswith("_test.py"):
        continue
    files.append(filename)

for filename in files:
    build_arg = "--build-arg=ALGO_FILE=examples/" + filename
    subprocess.run(
        ["docker", "build", "-t", basename.format(algo=filename.rstrip(".py")), "-f", "Dockerfile", build_arg, "."]
    )
