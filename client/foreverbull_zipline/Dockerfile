FROM lhjnilsson/zipline_base:latest

COPY foreverbull_zipline/src /app

ARG FB_WHEEL
COPY ${FB_WHEEL} /foreverbull-0.0.1-py3-none-any.whl
RUN pip install /foreverbull-0.0.1-py3-none-any.whl

RUN export PYTHONPATH="${PYTHONPATH}:/app/src"
WORKDIR /app
CMD ["python", "-m", "foreverbull_zipline"]
