FROM python:3.12

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

CMD [ "python", "-m", "_run_instance", "algo.py"]
