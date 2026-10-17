FROM python:3.14

ARG TARGETARCH

RUN apt-get update
RUN apt-get install -y libhdf5-dev liblapack-dev gfortran cmake build-essential python3-dev automake autoconf
RUN apt install -y libatlas-base-dev python3-dev gfortran pkg-config libfreetype6-dev hdf5-tools
RUN ln -s /usr/include/locale.h /usr/include/xlocale.h

RUN curl -L -o ta-lib-0.6.4-src.tar.gz  https://github.com/ta-lib/ta-lib/releases/download/v0.6.4/ta-lib-0.6.4-src.tar.gz
RUN tar -xzf ta-lib-0.6.4-src.tar.gz
WORKDIR /ta-lib-0.6.4/
RUN ./configure --prefix=/usr
RUN make
RUN make install
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
    echo "Unsupported architecture"; \ exit 1; \
    fi

RUN pip install -U .


RUN pip install zipline-reloaded --upgrade
RUN pip install psycopg2-binary --upgrade
