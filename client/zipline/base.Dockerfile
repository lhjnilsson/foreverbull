FROM python:3.14-trixie

ARG TARGETARCH

RUN apt-get update
RUN apt-get install -y libhdf5-dev liblapack-dev gfortran cmake build-essential python3-dev automake autoconf
RUN apt install -y libopenblas-dev python3-dev gfortran pkg-config libfreetype6-dev hdf5-tools
RUN ln -s /usr/include/locale.h /usr/include/xlocale.h

RUN apt upgrade -y


RUN pip install "Cython>=3.0.0,<4.0"
RUN pip install setuptools_scm --upgrade
RUN pip install toml --upgrade
RUN pip install numpy --upgrade
RUN apt install git -y
WORKDIR /
RUN git clone https://github.com/stefan-jansen/bcolz-zipline.git
WORKDIR /bcolz-zipline

RUN if [ "${TARGETARCH}" = "arm64" ]; then \
    DISABLE_BCOLZ_AVX2=1 DISABLE_BCOLZ_SSE2=1 make build; \
    elif [ "${TARGETARCH}" = "amd64" ]; then \
    make build; \
    else \
    echo "Unsupported architecture: ${TARGETARCH}" >&2; \
    exit 1; \
    fi

RUN pip install -U .


RUN pip install zipline-reloaded --upgrade
# SQLAlchemy 2.1 maps postgresql:// to psycopg 3. The zipline package depends
# on SQLAlchemy and psycopg. This base image installs the driver for ingest.
RUN pip install "psycopg[binary]" --upgrade

# Build steps above stay root. The image user is the one a derived runtime inherits.
RUN groupadd --gid 10001 foreverbull \
 && useradd --uid 10001 --gid 10001 --create-home --home-dir /home/foreverbull --shell /usr/sbin/nologin foreverbull \
 && chmod 1777 /tmp
USER foreverbull
HEALTHCHECK --interval=30s --timeout=10s --retries=3 CMD python -c "import importlib.util, sys; sys.exit(0 if importlib.util.find_spec('zipline') else 1)"
