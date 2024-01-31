FROM python:3.11

RUN apt-get update
RUN apt-get install -y libhdf5-dev liblapack-dev gfortran cmake build-essential python3-dev automake autoconf
RUN apt install -y libatlas-base-dev python3-dev gfortran pkg-config libfreetype6-dev hdf5-tools
RUN ln -s /usr/include/locale.h /usr/include/xlocale.h

RUN curl -L -o ta-lib-0.4.0-src.tar.gz  http://prdownloads.sourceforge.net/ta-lib/ta-lib-0.4.0-src.tar.gz 
RUN tar xvfz ta-lib-0.4.0-src.tar.gz
WORKDIR /ta-lib
RUN cp /usr/share/automake-1.16/config.guess .
RUN ./configure --prefix=/usr/local
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
RUN make build
RUN pip install -U .

WORKDIR /app
COPY . .

RUN pip install -e .

RUN pip install zipline-reloaded --upgrade
RUN pip install psycopg2-binary --upgrade

RUN export PYTHONPATH="${PYTHONPATH}:/app/src"

CMD ["python", "-m", "foreverbull_zipline"]