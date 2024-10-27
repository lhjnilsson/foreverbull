from multiprocessing import Event
from threading import Thread

import pytest

from foreverbull import exceptions
from foreverbull import worker
from foreverbull.pb.foreverbull.service import worker_pb2


class TestWorkerInstance:
    def test_configure_bad_file(self):
        w = worker.WorkerInstance("bad_file")
        with pytest.raises(exceptions.ConfigurationError):
            w.configure_execution(worker_pb2.ExecutionConfiguration())

    def test_configure_bad_broker_port(self, parallel_algo_file):
        algorithm, request, _ = parallel_algo_file
        w = worker.WorkerInstance(algorithm._file_path)
        request.brokerPort = 1234
        with pytest.raises(exceptions.ConfigurationError):
            w.configure_execution(request)

    def test_configure_bad_namespace_port(self, parallel_algo_file):
        algorithm, request, _ = parallel_algo_file
        w = worker.WorkerInstance(algorithm._file_path)
        request.namespacePort = 1234
        with pytest.raises(exceptions.ConfigurationError):
            w.configure_execution(request)

    def test_configure_bad_database(self, parallel_algo_file):
        algorithm, request, _ = parallel_algo_file
        w = worker.WorkerInstance(algorithm._file_path)
        request.databaseURL = "bad_url"
        with pytest.raises(exceptions.ConfigurationError):
            w.configure_execution(request)

    def test_configure_bad_parameters(self, parallel_algo_file):
        algorithm, request, _ = parallel_algo_file
        w = worker.WorkerInstance(algorithm._file_path)
        request.databaseURL = "bad_url"
        with pytest.raises(exceptions.ConfigurationError):
            w.configure_execution(request)

    @pytest.mark.skip()
    def test_configure_and_run_execution(self, namespace_server, parallel_algo_file):
        algorithm, request, process_symbols = parallel_algo_file
        w = worker.WorkerInstance(algorithm._file_path)
        w.configure_execution(request)
        t = Thread(target=process_symbols, args=())
        t.start()
        w.run_execution(
            Event(),
        )


class TestWorkerPool:
    def test_bad_algorithm_file_path(self):
        pool = worker.WorkerPool("bad_file")
        with pytest.raises(exceptions.ConfigurationError):
            with pool:
                pass

    def test_configure_not_running(self, parallel_algo_file):
        algorithm, _, _ = parallel_algo_file
        pool = worker.WorkerPool(algorithm._file_path)
        with pytest.raises(RuntimeError):
            pool.configure_execution(worker_pb2.ExecutionConfiguration())

    @pytest.mark.parametrize(
        "broker_port,namespace_port,database_url,expected_exception",
        [
            (1234, None, None, exceptions.ConfigurationError),
            (None, 1234, None, exceptions.ConfigurationError),
            (None, None, "bad_url", exceptions.ConfigurationError),
            (None, None, None, None),
        ],
    )
    def test_configure(
        self,
        namespace_server,
        parallel_algo_file,
        broker_port,
        namespace_port,
        database_url,
        expected_exception,
    ):
        algorithm, request, process_symbols = parallel_algo_file
        if broker_port:
            request.brokerPort = broker_port
        if namespace_port:
            request.namespacePort = namespace_port
        if database_url:
            request.databaseURL = database_url
        pool = worker.WorkerPool(algorithm._file_path)
        with pool:
            if expected_exception:
                with pytest.raises(expected_exception):
                    pool.configure_execution(request)
            else:
                pool.configure_execution(request)

    def test_run_not_running(self, parallel_algo_file):
        algorithm, _, _ = parallel_algo_file
        pool = worker.WorkerPool(algorithm._file_path)
        with pytest.raises(RuntimeError):
            pool.run_execution(Event())

    def test_run_not_configured(self, namespace_server, parallel_algo_file):
        algorithm, request, _ = parallel_algo_file
        pool = worker.WorkerPool(algorithm._file_path)
        with pool:
            with pytest.raises(exceptions.ConfigurationError):
                pool.run_execution(Event())

    def test_run_stop_before_process(self, namespace_server, parallel_algo_file):
        algorithm, request, process_symbols = parallel_algo_file
        pool = worker.WorkerPool(algorithm._file_path)
        with pool:
            pool.configure_execution(request)
            stop_event = Event()
            pool.run_execution(stop_event)
            stop_event.set()

    def test_run(self, namespace_server, parallel_algo_file):
        algorithm, request, process_symbols = parallel_algo_file
        pool = worker.WorkerPool(algorithm._file_path)
        with pool:
            stop_event = Event()
            pool.configure_execution(request)
            pool.run_execution(stop_event)
            orders = process_symbols()
            stop_event.set()
            assert orders
