FROM python:3.11

RUN apt update
RUN apt install -y cmake build-essential python3-dev

RUN pip install pip --upgrade
RUN pip install setuptools --upgrade

WORKDIR /app
COPY . .

RUN pip install -e .

RUN export PYTHONPATH="${PYTHONPATH}:/app/src"

ARG ALGO_FILE
RUN mkdir /algos
RUN cp ${ALGO_FILE} /algos/algo.py

WORKDIR /algos

CMD [ "python", "-m", "foreverbull", "/app/src/foreverbull/_run_instance.py", "algo.py"]
