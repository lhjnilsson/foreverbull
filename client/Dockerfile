FROM python:3.14

RUN apt update
RUN apt install -y cmake build-essential python3-dev

RUN pip install pip --upgrade
RUN pip install setuptools --upgrade

WORKDIR /app

ARG FB_WHEEL
COPY ${FB_WHEEL} /foreverbull-0.0.1-py3-none-any.whl
RUN pip install /foreverbull-0.0.1-py3-none-any.whl

ARG ALGO_FILE
RUN mkdir /algos
COPY ${ALGO_FILE} /algos/algo.py
COPY foreverbull/src/foreverbull/_run_instance.py /algos/_run_instance.py

WORKDIR /algos

# The worker dials the broker. It has no listen port, so the check is that pid 1 is alive.
RUN groupadd --gid 10001 foreverbull \
 && useradd --uid 10001 --gid 10001 --create-home --home-dir /home/foreverbull --shell /usr/sbin/nologin foreverbull \
 && chown -R foreverbull:foreverbull /algos
USER foreverbull
HEALTHCHECK --interval=30s --timeout=3s --retries=3 CMD python -c "import os; os.kill(1, 0)"

CMD [ "python", "-m", "_run_instance", "algo.py"]
