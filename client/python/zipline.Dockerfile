FROM lhjnilsson/zipline_base:latest

WORKDIR /app
COPY . .

RUN pip install -e .

RUN export PYTHONPATH="${PYTHONPATH}:/app/src"

CMD ["python", "-m", "foreverbull_zipline"]
